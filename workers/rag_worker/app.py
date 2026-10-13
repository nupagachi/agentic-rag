"""Worker lifecycle: check Redis, log ``ready``, wait for SIGINT/SIGTERM, log ``shutdown``.

The queue runtime (consumers, retry, DLQ) arrives with M4-02; this is only the skeleton.
"""

from __future__ import annotations

import asyncio
import contextlib
import functools
import signal
import time
from collections.abc import Callable
from typing import Protocol

from redis.asyncio import Redis
from redis.exceptions import AuthenticationError

from rag_worker.config import Settings
from rag_worker.logging import get_logger

REDIS_CONNECT_TIMEOUT_S = 5.0
_RETRY_INTERVAL_S = 0.5

EXIT_OK = 0
EXIT_ERROR = 1


class RedisProbe(Protocol):
    """The slice of a Redis client the skeleton needs."""

    async def ping(self) -> None: ...

    async def aclose(self) -> None: ...


ProbeFactory = Callable[[str], RedisProbe]


class _RedisProbe:
    def __init__(self, url: str) -> None:
        # Short per-attempt timeouts; the overall deadline is enforced by wait_for_redis.
        self._client = Redis.from_url(url, socket_connect_timeout=2, socket_timeout=2)

    async def ping(self) -> None:
        await self._client.ping()

    async def aclose(self) -> None:
        await self._client.aclose()


def default_probe_factory(url: str) -> RedisProbe:
    return _RedisProbe(url)


def install_signal_handlers(stop: asyncio.Event) -> Callable[[], None]:
    """Make SIGINT/SIGTERM (and SIGBREAK on Windows) set ``stop``.

    POSIX uses ``loop.add_signal_handler``. Windows event loops do not implement it, so we
    fall back to ``signal.signal`` and hop into the loop with ``call_soon_threadsafe``.
    Returns a function that restores the previous handlers.
    """
    loop = asyncio.get_running_loop()
    restore: list[Callable[[], object]] = []

    def request_stop() -> None:
        loop.call_soon_threadsafe(stop.set)

    for name in ("SIGINT", "SIGTERM", "SIGBREAK"):
        sig = getattr(signal, name, None)
        if sig is None:
            continue
        try:
            loop.add_signal_handler(sig, stop.set)
        except (NotImplementedError, RuntimeError):
            previous = signal.signal(sig, lambda _signum, _frame: request_stop())
            restore.append(functools.partial(signal.signal, sig, previous))
        else:
            restore.append(functools.partial(loop.remove_signal_handler, sig))

    def restore_all() -> None:
        for undo in restore:
            undo()

    return restore_all


async def wait_for_redis(
    probe: RedisProbe,
    stop: asyncio.Event,
    *,
    timeout_s: float = REDIS_CONNECT_TIMEOUT_S,
) -> bool:
    """Ping Redis, retrying until ``timeout_s`` has passed. False on failure or stop request.

    Never logs the exception message: client libraries may include connection details.
    """
    log = get_logger()
    deadline = time.monotonic() + timeout_s
    last_error = "timeout"
    while not stop.is_set():
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            break
        try:
            async with asyncio.timeout(remaining):
                await probe.ping()
        except AuthenticationError as exc:  # wrong credentials: retrying cannot help
            last_error = type(exc).__name__
            break
        except Exception as exc:  # any other failure means "not reachable yet"
            last_error = type(exc).__name__
            with contextlib.suppress(TimeoutError):
                await asyncio.wait_for(stop.wait(), min(_RETRY_INTERVAL_S, max(remaining, 0)))
        else:
            return True
    if not stop.is_set():
        log.error("redis_unavailable", reason=last_error, timeout_s=timeout_s)
    return False


async def run(
    settings: Settings,
    *,
    stop: asyncio.Event | None = None,
    probe_factory: ProbeFactory = default_probe_factory,
    connect_timeout_s: float = REDIS_CONNECT_TIMEOUT_S,
) -> int:
    """Run the worker until a stop signal. Returns the process exit code."""
    log = get_logger()
    stop = stop or asyncio.Event()
    restore_signals = install_signal_handlers(stop)
    probe: RedisProbe | None = None
    try:
        log.info("starting", config=settings.redacted())
        try:
            probe = probe_factory(settings.redis_url.get_secret_value())
        except Exception as exc:
            # Never log the message or traceback: they can echo parts of the URL (SR-7).
            log.error("redis_invalid", reason=type(exc).__name__)
            return EXIT_ERROR
        if not await wait_for_redis(probe, stop, timeout_s=connect_timeout_s):
            if stop.is_set():
                log.info("shutdown", reason="signal_during_startup")
                return EXIT_OK
            return EXIT_ERROR
        log.info("ready", redis=settings.redis_target())
        await stop.wait()
        log.info("shutdown", reason="signal")
        return EXIT_OK
    finally:
        if probe is not None:
            await probe.aclose()
        restore_signals()

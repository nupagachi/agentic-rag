from __future__ import annotations

import asyncio
import io
import json
import signal
import sys

import pytest
from redis.exceptions import AuthenticationError

from rag_worker.app import install_signal_handlers, run, wait_for_redis
from rag_worker.config import Settings
from rag_worker.logging import configure_logging
from tests.conftest import REDIS_URL, SENTINEL


class FakeProbe:
    """Stands in for Redis: fails ``fail_times`` times, then answers."""

    def __init__(self, fail_times: int = 0, hang: bool = False, auth_error: bool = False) -> None:
        self.auth_error = auth_error
        self.fail_times = fail_times
        self.hang = hang
        self.pings = 0
        self.closed = False

    async def ping(self) -> None:
        self.pings += 1
        if self.hang:
            await asyncio.sleep(3600)
        if self.auth_error:
            raise AuthenticationError(f"bad password {SENTINEL}")
        if self.pings <= self.fail_times:
            raise ConnectionError(f"cannot reach {REDIS_URL}")  # message must never be logged

    async def aclose(self) -> None:
        self.closed = True


def _settings(monkeypatch: pytest.MonkeyPatch) -> Settings:
    monkeypatch.setenv("REDIS_URL", REDIS_URL)
    return Settings()  # type: ignore[call-arg]


def _records(stream: io.StringIO) -> list[dict[str, object]]:
    return [json.loads(line) for line in stream.getvalue().splitlines()]


@pytest.fixture
def log_stream() -> io.StringIO:
    stream = io.StringIO()
    configure_logging("info", "idle", stream=stream)
    return stream


def test_ready_then_shutdown_on_stop_event(
    monkeypatch: pytest.MonkeyPatch, log_stream: io.StringIO
) -> None:
    settings = _settings(monkeypatch)
    probe = FakeProbe()

    async def scenario() -> int:
        stop = asyncio.Event()
        asyncio.get_running_loop().call_later(0.05, stop.set)
        return await run(settings, stop=stop, probe_factory=lambda _url: probe)

    assert asyncio.run(scenario()) == 0

    records = _records(log_stream)
    assert [r["msg"] for r in records] == ["starting", "ready", "shutdown"]
    for record in records:
        assert {"ts", "level", "msg", "stage"} <= record.keys()
        assert record["stage"] == "idle"
    assert probe.closed
    assert SENTINEL not in log_stream.getvalue()


@pytest.mark.parametrize("sig_name", ["SIGINT", "SIGTERM"])
def test_wait_loop_stops_on_signal(
    monkeypatch: pytest.MonkeyPatch, log_stream: io.StringIO, sig_name: str
) -> None:
    sig = getattr(signal, sig_name)
    settings = _settings(monkeypatch)
    before = signal.getsignal(sig)

    async def scenario() -> int:
        asyncio.get_running_loop().call_later(0.1, signal.raise_signal, sig)
        return await asyncio.wait_for(
            run(settings, probe_factory=lambda _url: FakeProbe()), timeout=5
        )

    assert asyncio.run(scenario()) == 0
    assert _records(log_stream)[-1]["msg"] == "shutdown"
    assert signal.getsignal(sig) == before  # handlers restored


@pytest.mark.skipif(sys.platform != "win32", reason="Windows fallback path")
def test_windows_fallback_uses_signal_module() -> None:
    async def scenario() -> bool:
        stop = asyncio.Event()
        restore = install_signal_handlers(stop)
        try:
            signal.raise_signal(signal.SIGBREAK)
            await asyncio.wait_for(stop.wait(), timeout=2)
        finally:
            restore()
        return stop.is_set()

    assert asyncio.run(scenario())


def test_redis_down_exits_1_without_leaking_url(
    monkeypatch: pytest.MonkeyPatch, log_stream: io.StringIO
) -> None:
    settings = _settings(monkeypatch)
    probe = FakeProbe(fail_times=10_000)

    code = asyncio.run(run(settings, probe_factory=lambda _url: probe, connect_timeout_s=0.3))

    assert code == 1
    records = _records(log_stream)
    error = records[-1]
    assert error["level"] == "error"
    assert error["msg"] == "redis_unavailable"
    assert error["reason"] == "ConnectionError"
    assert SENTINEL not in log_stream.getvalue()
    assert "ready" not in [r["msg"] for r in records]
    assert probe.closed


def test_hanging_redis_times_out(log_stream: io.StringIO) -> None:
    async def scenario() -> bool:
        return await wait_for_redis(FakeProbe(hang=True), asyncio.Event(), timeout_s=0.2)

    assert asyncio.run(scenario()) is False
    assert _records(log_stream)[-1]["reason"] == "TimeoutError"


def test_redis_recovers_within_deadline(log_stream: io.StringIO) -> None:
    probe = FakeProbe(fail_times=1)

    async def scenario() -> bool:
        return await wait_for_redis(probe, asyncio.Event(), timeout_s=3)

    assert asyncio.run(scenario()) is True
    assert probe.pings == 2


def test_stop_during_connect_exits_0(
    monkeypatch: pytest.MonkeyPatch, log_stream: io.StringIO
) -> None:
    settings = _settings(monkeypatch)

    async def scenario() -> int:
        stop = asyncio.Event()
        asyncio.get_running_loop().call_later(0.1, stop.set)
        return await run(
            settings, stop=stop, probe_factory=lambda _url: FakeProbe(fail_times=10_000)
        )

    assert asyncio.run(scenario()) == 0
    assert _records(log_stream)[-1]["msg"] == "shutdown"


def test_authentication_error_is_not_retried(
    monkeypatch: pytest.MonkeyPatch, log_stream: io.StringIO
) -> None:
    settings = _settings(monkeypatch)
    probe = FakeProbe(auth_error=True)

    code = asyncio.run(run(settings, probe_factory=lambda _url: probe, connect_timeout_s=5))

    assert code == 1
    assert probe.pings == 1
    error = _records(log_stream)[-1]
    assert error["msg"] == "redis_unavailable"
    assert error["reason"] == "AuthenticationError"
    assert SENTINEL not in log_stream.getvalue()


def test_probe_factory_failure_exits_1_and_restores_signals(
    monkeypatch: pytest.MonkeyPatch, log_stream: io.StringIO
) -> None:
    settings = _settings(monkeypatch)
    before = signal.getsignal(signal.SIGINT)

    def broken_factory(url: str) -> FakeProbe:
        raise TypeError(f"unexpected keyword argument in {url}")

    assert asyncio.run(run(settings, probe_factory=broken_factory)) == 1

    error = _records(log_stream)[-1]
    assert error["msg"] == "redis_invalid"
    assert error["reason"] == "TypeError"
    assert "exception" not in error
    assert SENTINEL not in log_stream.getvalue()
    assert signal.getsignal(signal.SIGINT) == before

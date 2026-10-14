"""Worker skeleton against a real Redis (dev stack). Marked ``integration``, so not in ``check``."""

from __future__ import annotations

from collections.abc import Callable

import pytest

from tests.integration.harness import (
    DEAD_REDIS_URL,
    SENTINEL,
    SHUTDOWN_TIMEOUT_S,
    STARTUP_TIMEOUT_S,
    Worker,
    password_of,
    require_redis,
    with_password,
)

pytestmark = pytest.mark.integration

StartWorker = Callable[[str], Worker]


def test_worker_logs_ready_then_exits_0_on_interrupt(start_worker: StartWorker) -> None:
    """AC4 and SR-2: right password -> ``ready`` within 10s; Ctrl+C -> exit 0 within 10s."""
    redis_url = require_redis()
    worker = start_worker(redis_url)

    ready = worker.wait_for_record("ready", STARTUP_TIMEOUT_S)
    assert ready is not None, f"no `ready` log line within 10s; output:\n{worker.output()}"
    assert ready["level"] == "info"
    assert "@" not in ready["redis"], "ready line must show the Redis target without credentials"

    worker.interrupt()

    code = worker.wait_exit(SHUTDOWN_TIMEOUT_S)

    output = worker.output()
    assert code == 0, f"exit code {code}; output:\n{output}"
    assert '"shutdown"' in output
    password = password_of(redis_url)
    if password:
        assert password not in output


def test_worker_exits_1_without_leaking_password_when_redis_is_unreachable(
    start_worker: StartWorker,
) -> None:
    """AC5 and SR-7: nothing listens on the port -> exit 1 within 10s, no password in output.

    Needs no running stack, so it never skips.
    """
    worker = start_worker(DEAD_REDIS_URL)

    code = worker.wait_exit(SHUTDOWN_TIMEOUT_S)

    output = worker.output()
    assert code == 1, f"exit code {code}; output:\n{output}"
    assert SENTINEL not in output
    assert '"redis_unavailable"' in output
    assert '"ready"' not in output


def test_worker_exits_1_without_leaking_password_when_redis_password_is_wrong(
    start_worker: StartWorker,
) -> None:
    """SR-2 and SR-7: wrong password -> exit 1 within 10s; neither password appears in output."""
    redis_url = require_redis()
    wrong = f"wrong-{SENTINEL}"
    worker = start_worker(with_password(redis_url, wrong))

    code = worker.wait_exit(SHUTDOWN_TIMEOUT_S)

    output = worker.output()
    assert code == 1, f"exit code {code}; output:\n{output}"
    assert wrong not in output
    real = password_of(redis_url)
    if real:
        assert real not in output
    assert '"ready"' not in output

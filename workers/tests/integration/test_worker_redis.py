"""Worker skeleton against a real Redis (dev stack). Marked ``integration``, so not in ``check``.

Leak checks are written as ``leaked = ...; assert not leaked`` on purpose: pytest prints both
sides of a failing ``assert secret not in output``, which would put the password in the log.
Failure messages that show the process output go through ``printable_output``.
"""

from __future__ import annotations

from collections.abc import Callable

import pytest

from tests.integration.harness import (
    DEAD_REDIS_URL,
    SENTINEL,
    SHUTDOWN_TIMEOUT_S,
    STARTUP_TIMEOUT_S,
    Worker,
    passwords_of,
    redact,
    require_redis,
    with_password,
)

pytestmark = pytest.mark.integration

StartWorker = Callable[..., Worker]


def test_worker_logs_ready_then_exits_0_on_interrupt(start_worker: StartWorker) -> None:
    """AC4 and SR-2: right password -> ``ready`` within 10s; Ctrl+C -> exit 0 within 10s."""
    redis_url = require_redis()
    secrets = passwords_of(redis_url)
    worker = start_worker(redis_url)

    ready = worker.wait_for_record("ready", STARTUP_TIMEOUT_S)
    assert ready is not None, f"no `ready` line within 10s; output:\n{worker.printable_output()}"
    assert ready["level"] == "info"
    target = redact(str(ready["redis"]), *secrets)
    has_userinfo = "@" in str(ready["redis"])
    assert not has_userinfo, f"ready line shows credentials in the Redis target: {target}"

    worker.interrupt()

    code = worker.wait_exit(SHUTDOWN_TIMEOUT_S)

    output = worker.output()
    assert code == 0, f"exit code {code}; output:\n{redact(output, *secrets)}"
    assert '"shutdown"' in output
    leaked = any(secret in output for secret in secrets)
    assert not leaked, "worker output contains the Redis password"


def test_worker_exits_1_without_leaking_password_when_redis_is_unreachable(
    start_worker: StartWorker,
) -> None:
    """AC5 and SR-7: nothing listens on the port -> exit 1 within 10s, no password in output.

    Needs no running stack, so it never skips.
    """
    worker = start_worker(DEAD_REDIS_URL)

    code = worker.wait_exit(SHUTDOWN_TIMEOUT_S)

    output = worker.output()
    assert code == 1, f"exit code {code}; output:\n{worker.printable_output()}"
    leaked = SENTINEL in output
    assert not leaked, "worker output contains the Redis password"
    assert '"redis_unavailable"' in output
    assert '"ready"' not in output


def test_worker_exits_1_without_leaking_password_when_redis_password_is_wrong(
    start_worker: StartWorker,
) -> None:
    """SR-2 and SR-7: wrong password -> exit 1 within 10s; neither password appears in output."""
    redis_url = require_redis()
    wrong = f"wrong-{SENTINEL}"
    secrets = [*passwords_of(redis_url), wrong]
    worker = start_worker(with_password(redis_url, wrong), *secrets)

    code = worker.wait_exit(SHUTDOWN_TIMEOUT_S)

    output = worker.output()
    assert code == 1, f"exit code {code}; output:\n{worker.printable_output()}"
    leaked = any(secret in output for secret in secrets)
    assert not leaked, "worker output contains a Redis password (the real or the wrong one)"
    assert '"ready"' not in output

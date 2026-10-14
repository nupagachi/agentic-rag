"""Helpers for the worker integration tests.

Decision (M0): these tests run against the dev stack from ``make up`` (Redis on
127.0.0.1:6379) instead of testcontainers, to keep M0 light. ``REDIS_URL`` comes from the
environment; ``make -C workers test-integration`` loads ``../.env`` first.

A test that needs Redis skips with an explicit reason when it is not configured or not
reachable. Set ``INTEGRATION_REQUIRE_STACK=1`` (CI) to turn that skip into a failure.
"""

from __future__ import annotations

import json
import os
import queue
import re
import signal
import socket
import subprocess
import sys
import threading
import time
from pathlib import Path
from typing import Any
from urllib.parse import quote, unquote, urlsplit

import pytest

REQUIRE_STACK_ENV = "INTEGRATION_REQUIRE_STACK"
WORKERS_DIR = Path(__file__).resolve().parents[2]
SENTINEL = "S3NTINEL"
DEAD_REDIS_URL = f"redis://:{SENTINEL}@127.0.0.1:1/0"

# tests/conftest.py removes REDIS_URL from the environment before every test. The real
# value is captured here, at import (collection) time, which runs before any fixture.
REAL_REDIS_URL = os.environ.get("REDIS_URL", "")

STARTUP_TIMEOUT_S = 10.0
SHUTDOWN_TIMEOUT_S = 10.0


def skip_or_fail(reason: str) -> None:
    if os.environ.get(REQUIRE_STACK_ENV) == "1":
        pytest.fail(f"{reason} ({REQUIRE_STACK_ENV}=1)")
    pytest.skip(reason)


def require_redis() -> str:
    """Return the real ``REDIS_URL`` or skip. Messages never include the URL (password)."""
    if not REAL_REDIS_URL:
        skip_or_fail(
            "integration stack not configured: REDIS_URL is not set; run `make up`, "
            "then `make -C workers test-integration` (it loads ../.env)"
        )
    try:
        parts = urlsplit(REAL_REDIS_URL)
        host, port = parts.hostname, parts.port or 6379
    except ValueError:
        skip_or_fail("integration stack not configured: REDIS_URL is not a valid URL")
        raise
    try:
        socket.create_connection((host or "", port), timeout=2).close()
    except OSError:
        skip_or_fail(
            f"integration stack not running: cannot reach redis at {host}:{port}; "
            "run `make up`, then `make -C workers test-integration`"
        )
    return REAL_REDIS_URL


def passwords_of(url: str) -> list[str]:
    """Password of a URL in both forms it can take in output: as written (percent-encoded)
    and decoded. Empty list when there is none.
    """
    encoded = urlsplit(url).password or ""  # urlsplit does not decode the userinfo
    if not encoded:
        return []
    decoded = unquote(encoded)
    return [encoded] if decoded == encoded else [encoded, decoded]


_USERINFO = re.compile(r"://[^/\s@]*@")


def redact(text: str, *secrets: str) -> str:
    """Make text safe to print in a failure: known secrets become ``***`` and any userinfo in
    a URL (``://user:pass@``) becomes ``://***@``.
    """
    for secret in secrets:
        if secret:
            text = text.replace(secret, "***")
    return _USERINFO.sub("://***@", text)


def with_password(url: str, password: str) -> str:
    """Return ``url`` with its password replaced (the user name is kept)."""
    parts = urlsplit(url)
    host = parts.hostname or ""
    if ":" in host:
        host = f"[{host}]"
    netloc = f"{quote(parts.username or '')}:{quote(password)}@{host}"
    if parts.port:
        netloc += f":{parts.port}"
    return parts._replace(netloc=netloc).geturl()


class Worker:
    """A running ``python -m rag_worker`` subprocess with its output captured."""

    def __init__(self, redis_url: str, *secrets: str) -> None:
        # Passwords that must never be printed by a failing test.
        self._secrets = [*passwords_of(redis_url), *secrets]
        env = {k: v for k, v in os.environ.items() if k not in {"REDIS_URL", "WORKER_LOG_LEVEL"}}
        env["REDIS_URL"] = redis_url
        env["PYTHONUNBUFFERED"] = "1"
        flags = subprocess.CREATE_NEW_PROCESS_GROUP if sys.platform == "win32" else 0  # type: ignore[attr-defined,unused-ignore]
        self._proc = subprocess.Popen(
            [sys.executable, "-m", "rag_worker"],
            cwd=WORKERS_DIR,
            env=env,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            encoding="utf-8",
            creationflags=flags,
        )
        self.lines: list[str] = []
        self._queue: queue.Queue[str | None] = queue.Queue()
        self._reader = threading.Thread(target=self._pump, daemon=True)
        self._reader.start()

    def _pump(self) -> None:
        stream = self._proc.stdout
        assert stream is not None
        for line in stream:
            self._queue.put(line.rstrip("\r\n"))
        self._queue.put(None)

    def wait_for_record(self, msg: str, timeout_s: float) -> dict[str, Any] | None:
        """Wait up to ``timeout_s`` for a JSON log line whose ``msg`` equals ``msg``."""
        deadline = time.monotonic() + timeout_s
        while (remaining := deadline - time.monotonic()) > 0:
            try:
                line = self._queue.get(timeout=remaining)
            except queue.Empty:
                return None
            if line is None:  # process closed its output
                return None
            self.lines.append(line)
            record = _parse(line)
            if record is not None and record.get("msg") == msg:
                return record
        return None

    def interrupt(self) -> None:
        """Ask for a graceful stop: Ctrl+C semantics (SIGINT; CTRL_BREAK on Windows)."""
        if sys.platform == "win32":
            self._proc.send_signal(signal.CTRL_BREAK_EVENT)  # type: ignore[attr-defined,unused-ignore]
        else:
            self._proc.send_signal(signal.SIGINT)

    def wait_exit(self, timeout_s: float) -> int | None:
        """Return the exit code, or None when the process is still running after the wait."""
        try:
            return self._proc.wait(timeout=timeout_s)
        except subprocess.TimeoutExpired:
            return None

    def output(self) -> str:
        """Everything printed so far (call after the process exited)."""
        self._reader.join(timeout=5)
        while True:
            try:
                line = self._queue.get_nowait()
            except queue.Empty:
                break
            if line is not None:
                self.lines.append(line)
        return "\n".join(self.lines)

    def printable_output(self) -> str:
        """``output()`` with known passwords and URL credentials masked, for failure messages."""
        return redact(self.output(), *self._secrets)

    def close(self) -> None:
        if self._proc.poll() is None:
            self._proc.kill()
            self._proc.wait(timeout=5)
        self._reader.join(timeout=5)
        if self._proc.stdout is not None:
            self._proc.stdout.close()


def _parse(line: str) -> dict[str, Any] | None:
    try:
        value = json.loads(line)
    except ValueError:
        return None
    return value if isinstance(value, dict) else None

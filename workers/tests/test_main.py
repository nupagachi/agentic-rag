from __future__ import annotations

import json

import pytest

from rag_worker.__main__ import main
from tests.conftest import REDIS_URL, SENTINEL


def test_missing_config_exits_1_and_names_variable(capsys: pytest.CaptureFixture[str]) -> None:
    assert main([]) == 1

    out = capsys.readouterr().out
    record = json.loads(out.splitlines()[-1])
    assert record["msg"] == "config_invalid"
    assert record["level"] == "error"
    assert record["stage"] == "idle"
    assert record["errors"] == [{"var": "REDIS_URL", "type": "missing"}]


def test_bad_url_exits_1_without_value(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setenv("REDIS_URL", f"mysql://:{SENTINEL}@127.0.0.1/0")

    assert main(["--stage", "parse"]) == 1

    captured = capsys.readouterr()
    assert SENTINEL not in captured.out + captured.err
    record = json.loads(captured.out.splitlines()[-1])
    assert record["stage"] == "parse"
    assert record["errors"][0]["var"] == "REDIS_URL"


def test_invalid_stage_name_is_rejected(capsys: pytest.CaptureFixture[str]) -> None:
    with pytest.raises(SystemExit) as info:
        main(["--stage", "Bad Stage!"])

    assert info.value.code == 2
    assert "--stage" in capsys.readouterr().err


def test_unreachable_redis_exits_1(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    # Port 1 on loopback: connection refused immediately; shorten the 5s deadline.
    monkeypatch.setenv("REDIS_URL", REDIS_URL.replace(":6379", ":1"))
    monkeypatch.setattr("rag_worker.__main__.run", _run_with_short_deadline)

    assert main([]) == 1

    out = capsys.readouterr().out
    assert SENTINEL not in out
    assert json.loads(out.splitlines()[-1])["msg"] == "redis_unavailable"


async def _run_with_short_deadline(settings: object) -> int:
    from rag_worker.app import run
    from rag_worker.config import Settings

    assert isinstance(settings, Settings)
    return await run(settings, connect_timeout_s=0.5)

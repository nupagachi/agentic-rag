from __future__ import annotations

import pytest
from pydantic import ValidationError

from rag_worker.config import describe_errors, load_settings, mask_redis_url
from tests.conftest import REDIS_URL, SENTINEL


def test_valid_config_from_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("REDIS_URL", REDIS_URL)
    monkeypatch.setenv("WORKER_LOG_LEVEL", "DEBUG")

    settings = load_settings()

    assert settings.redis_url.get_secret_value() == REDIS_URL
    assert settings.log_level == "debug"


def test_log_level_defaults_to_info(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("REDIS_URL", REDIS_URL)

    assert load_settings().log_level == "info"


def test_missing_redis_url_names_the_variable() -> None:
    with pytest.raises(ValidationError) as info:
        load_settings()

    assert describe_errors(info.value) == [{"var": "REDIS_URL", "type": "missing"}]


@pytest.mark.parametrize(
    "bad_url",
    [
        f"http://:{SENTINEL}@127.0.0.1:6379/0",
        f"redis://:{SENTINEL}@:6379/0",
        f"redis://:{SENTINEL}@127.0.0.1:notaport/0",
        f"{SENTINEL}",
        "",
        f"redis://:{SENTINEL}@127.0.0.1:6379/0?socket_timeout={SENTINEL}",
        f"redis://:{SENTINEL}@127.0.0.1:6379/0?db={SENTINEL}",
        f"redis://:{SENTINEL}@127.0.0.1:6379/0?nosuchoption={SENTINEL}",
        f"redis://:{SENTINEL}@127.0.0.1:6379/{SENTINEL}",
        f"redis://:{SENTINEL}@127.0.0.1:6379/0/{SENTINEL}",
        f"redis://:{SENTINEL}@127.0.0.1:6379/123",
        f"unix://:{SENTINEL}@/tmp/redis.sock",
    ],
)
def test_malformed_redis_url_is_rejected_without_echoing_value(
    monkeypatch: pytest.MonkeyPatch, bad_url: str
) -> None:
    monkeypatch.setenv("REDIS_URL", bad_url)

    with pytest.raises(ValidationError) as info:
        load_settings()

    errors = describe_errors(info.value)
    assert [e["var"] for e in errors] == ["REDIS_URL"]
    assert SENTINEL not in str(info.value)
    assert SENTINEL not in repr(errors)


def test_invalid_log_level_names_the_variable(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("REDIS_URL", REDIS_URL)
    monkeypatch.setenv("WORKER_LOG_LEVEL", "verbose")

    with pytest.raises(ValidationError) as info:
        load_settings()

    assert [e["var"] for e in describe_errors(info.value)] == ["WORKER_LOG_LEVEL"]


def test_secret_is_masked_everywhere(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("REDIS_URL", REDIS_URL)
    settings = load_settings()

    for rendered in (
        repr(settings),
        str(settings),
        settings.model_dump_json(),
        str(settings.redacted()),
    ):
        assert SENTINEL not in rendered
    assert settings.redis_target() == "redis://127.0.0.1:6379/0"


def test_mask_redis_url_drops_credentials_and_query() -> None:
    assert (
        mask_redis_url(f"rediss://user:{SENTINEL}@cache.local:6380/2?x=1")
        == "rediss://cache.local:6380/2"
    )
    assert mask_redis_url("redis://[::1]:6379/0") == "redis://[::1]:6379/0"
    assert mask_redis_url("redis://h:6379") == "redis://h:6379"
    assert SENTINEL not in mask_redis_url(f"redis://h:6379/{SENTINEL}")
    assert SENTINEL not in mask_redis_url(f"redis://:{SENTINEL}@h:badport/0")

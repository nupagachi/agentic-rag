"""Worker configuration, read from environment variables (and ``../.env`` in dev).

Security (SR-7): values never appear in validation errors (``hide_input_in_errors``), the
Redis URL is a ``SecretStr`` and only a masked form may be logged.
"""

from __future__ import annotations

from typing import Literal
from urllib.parse import urlsplit

from pydantic import AliasChoices, Field, SecretStr, ValidationError, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

LogLevel = Literal["debug", "info", "warning", "error"]

_REDIS_SCHEMES = frozenset({"redis", "rediss"})


class Settings(BaseSettings):
    """Validated at startup; the process exits with code 1 when this fails."""

    model_config = SettingsConfigDict(
        env_file="../.env",
        env_file_encoding="utf-8",
        extra="ignore",  # .env also holds variables of other apps
        hide_input_in_errors=True,
        frozen=True,
    )

    redis_url: SecretStr = Field(validation_alias=AliasChoices("REDIS_URL"))
    log_level: LogLevel = Field(default="info", validation_alias=AliasChoices("WORKER_LOG_LEVEL"))

    @field_validator("redis_url")
    @classmethod
    def _check_redis_url(cls, value: SecretStr) -> SecretStr:
        # Static messages only: the URL contains the password and must not be echoed.
        try:
            parts = urlsplit(value.get_secret_value())
            hostname = parts.hostname
            _ = parts.port  # raises ValueError on a malformed port
        except ValueError:
            raise ValueError("must be a redis:// or rediss:// URL") from None
        if parts.scheme not in _REDIS_SCHEMES or not hostname:
            raise ValueError("must be a redis:// or rediss:// URL")
        return value

    @field_validator("log_level", mode="before")
    @classmethod
    def _normalize_level(cls, value: object) -> object:
        return value.strip().lower() if isinstance(value, str) else value

    def redis_target(self) -> str:
        """Redis address without credentials, safe to log (``host:port/db``)."""
        return mask_redis_url(self.redis_url.get_secret_value())

    def redacted(self) -> dict[str, str]:
        """Config in a form that is safe to log (no secrets)."""
        return {"redis": self.redis_target(), "log_level": self.log_level}


def mask_redis_url(url: str) -> str:
    """Return ``host:port/db`` of a Redis URL, dropping user, password and query."""
    try:
        parts = urlsplit(url)
        host = parts.hostname or "?"
        port = f":{parts.port}" if parts.port else ""
    except ValueError:
        return "<invalid>"
    if ":" in host:  # IPv6 literal
        host = f"[{host}]"
    scheme = parts.scheme if parts.scheme in _REDIS_SCHEMES else "?"
    return f"{scheme}://{host}{port}{parts.path}"


def describe_errors(exc: ValidationError) -> list[dict[str, str]]:
    """Reduce a ``ValidationError`` to variable name + error type (never the input value)."""
    return [
        {"var": ".".join(str(part) for part in err["loc"]), "type": err["type"]}
        for err in exc.errors(include_input=False, include_url=False, include_context=False)
    ]


def load_settings() -> Settings:
    """Read settings from the environment; raises ``ValidationError`` when invalid."""
    return Settings()

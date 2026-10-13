"""Worker configuration, read from environment variables only.

In dev the Makefile loads the repo-root env file into the environment (``make dev``), so this
module never reads a file by itself and cannot pick up the wrong one from another CWD.

Security (SR-7): values never appear in validation errors (``hide_input_in_errors``), the
Redis URL is a ``SecretStr`` and only a masked form may be logged.
"""

from __future__ import annotations

import re
from typing import Any, Literal
from urllib.parse import urlsplit

from pydantic import Field, SecretStr, ValidationError, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict
from redis.asyncio.connection import Connection, parse_url

LogLevel = Literal["debug", "info", "warning", "error"]

_REDIS_SCHEMES = frozenset({"redis", "rediss"})
_DB_PATH = re.compile(r"/?(\d{0,2})")
_URL_ERROR = "must be a redis:// or rediss:// URL"


class Settings(BaseSettings):
    """Validated at startup; the process exits with code 1 when this fails."""

    model_config = SettingsConfigDict(
        extra="ignore",  # the environment holds variables of other apps
        hide_input_in_errors=True,
        frozen=True,
    )

    redis_url: SecretStr = Field(validation_alias="REDIS_URL")
    log_level: LogLevel = Field(default="info", validation_alias="WORKER_LOG_LEVEL")

    @field_validator("redis_url")
    @classmethod
    def _check_redis_url(cls, value: SecretStr) -> SecretStr:
        # Static messages only: the URL contains the password and must not be echoed.
        url = value.get_secret_value()
        try:
            parts = urlsplit(url)
            _ = parts.port  # raises ValueError on a malformed port
            if parts.scheme not in _REDIS_SCHEMES or not parts.hostname:
                raise ValueError(_URL_ERROR)
            if not _DB_PATH.fullmatch(parts.path):
                raise ValueError(_URL_ERROR)
            # Same parsing as Redis.from_url (incl. query values); building a Connection object
            # (no I/O) also rejects unknown query options.
            kwargs: dict[str, Any] = dict(parse_url(url))
            connection_class = kwargs.pop("connection_class", Connection)
            connection_class(**kwargs)
        except (ValueError, TypeError):
            raise ValueError(_URL_ERROR) from None
        return value

    @field_validator("log_level", mode="before")
    @classmethod
    def _normalize_level(cls, value: object) -> object:
        return value.strip().lower() if isinstance(value, str) else value

    def redis_target(self) -> str:
        """Redis address without credentials, safe to log (``scheme://host:port/db``)."""
        return mask_redis_url(self.redis_url.get_secret_value())

    def redacted(self) -> dict[str, str]:
        """Config in a form that is safe to log (no secrets)."""
        return {"redis": self.redis_target(), "log_level": self.log_level}


def mask_redis_url(url: str) -> str:
    """Return ``scheme://host:port/db`` of a Redis URL, dropping user, password and query.

    Only a numeric db is echoed; anything else in the path is dropped.
    """
    try:
        parts = urlsplit(url)
        host = parts.hostname or "?"
        port = f":{parts.port}" if parts.port else ""
    except ValueError:
        return "<invalid>"
    if ":" in host:  # IPv6 literal
        host = f"[{host}]"
    scheme = parts.scheme if parts.scheme in _REDIS_SCHEMES else "?"
    match = _DB_PATH.fullmatch(parts.path)
    db = f"/{match.group(1)}" if match and match.group(1) else ""
    return f"{scheme}://{host}{port}{db}"


def describe_errors(exc: ValidationError) -> list[dict[str, str]]:
    """Reduce a ``ValidationError`` to variable name + error type (never the input value)."""
    return [
        {"var": ".".join(str(part) for part in err["loc"]), "type": err["type"]}
        for err in exc.errors(include_input=False, include_url=False, include_context=False)
    ]


def load_settings() -> Settings:
    """Read settings from the environment; raises ``ValidationError`` when invalid."""
    return Settings()

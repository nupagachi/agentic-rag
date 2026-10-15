"""Đọc cấu hình từ biến môi trường (và file .env khi dev)."""

from functools import lru_cache
from typing import Literal

from pydantic import BaseModel, SecretStr
from pydantic_settings import BaseSettings, SettingsConfigDict

Stage = Literal["crawl", "parse", "chunk", "embed", "eval"]


class Log(BaseModel):
    level: str = "INFO"
    format: Literal["json", "text"] = "json"


class Postgres(BaseModel):
    dsn: SecretStr
    min_size: int = 1
    max_size: int = 5


class Redis(BaseModel):
    addr: str = "127.0.0.1:6379"
    password: SecretStr | None = None
    db: int = 0


class Config(BaseSettings):
    # Biến lồng dùng dấu "_": LOG_LEVEL, POSTGRES_DSN, REDIS_ADDR...
    model_config = SettingsConfigDict(
        env_file=".env",
        env_nested_delimiter="_",
        env_nested_max_split=1,
        extra="ignore",
    )

    app_env: str = "development"
    log: Log = Log()
    postgres: Postgres
    redis: Redis = Redis()

    @property
    def is_production(self) -> bool:
        return self.app_env == "production"


@lru_cache
def load() -> Config:
    return Config()  # type: ignore[call-arg]

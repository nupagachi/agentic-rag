"""Tạo Redis client."""

import redis

from rag_worker.platform.config import Config


def new(cfg: Config) -> redis.Redis:
    """redis-py kết nối lười nên không lỗi khi Redis chưa chạy."""
    host, _, port = cfg.redis.addr.rpartition(":")
    password = cfg.redis.password.get_secret_value() if cfg.redis.password else None
    return redis.Redis(
        host=host or "127.0.0.1",
        port=int(port),
        password=password,
        db=cfg.redis.db,
        decode_responses=True,
    )

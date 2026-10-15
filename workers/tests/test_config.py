from rag_worker.platform.config import Config


def test_nested_env(monkeypatch):
    monkeypatch.setenv("POSTGRES_DSN", "postgres://rag:pw@localhost:5432/rag")
    monkeypatch.setenv("REDIS_ADDR", "redis:6380")
    monkeypatch.setenv("LOG_LEVEL", "debug")

    cfg = Config(_env_file=None)

    assert cfg.postgres.dsn.get_secret_value().endswith("/rag")
    assert cfg.redis.addr == "redis:6380"
    assert cfg.log.level == "debug"
    assert cfg.log.format == "json"
    assert not cfg.is_production

"""Pool kết nối Postgres qua psycopg 3."""

from psycopg_pool import ConnectionPool

from rag_worker.platform.config import Config


def new(cfg: Config) -> ConnectionPool:
    """Mở pool nhưng không chờ kết nối: worker vẫn khởi động khi Postgres chưa sẵn sàng."""
    pool = ConnectionPool(
        cfg.postgres.dsn.get_secret_value(),
        min_size=cfg.postgres.min_size,
        max_size=cfg.postgres.max_size,
        open=False,
    )
    pool.open(wait=False)
    return pool


def ping(pool: ConnectionPool, timeout: float = 5.0) -> None:
    with pool.connection(timeout=timeout) as conn:
        conn.execute("SELECT 1")

from __future__ import annotations

import pytest

SENTINEL = "S3NTINEL"
REDIS_URL = f"redis://:{SENTINEL}@127.0.0.1:6379/0"


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch: pytest.MonkeyPatch) -> None:
    """Tests must not depend on the developer's shell."""
    monkeypatch.delenv("REDIS_URL", raising=False)
    monkeypatch.delenv("WORKER_LOG_LEVEL", raising=False)

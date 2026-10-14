from __future__ import annotations

from collections.abc import Callable, Iterator

import pytest

from tests.integration.harness import Worker


@pytest.fixture
def start_worker() -> Iterator[Callable[[str], Worker]]:
    """Start ``python -m rag_worker`` with a given REDIS_URL; always killed at teardown."""
    started: list[Worker] = []

    def start(redis_url: str) -> Worker:
        worker = Worker(redis_url)
        started.append(worker)
        return worker

    yield start
    for worker in started:
        worker.close()

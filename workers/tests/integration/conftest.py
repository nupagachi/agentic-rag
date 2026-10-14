from __future__ import annotations

from collections.abc import Callable, Iterator

import pytest

from tests.integration.harness import Worker


@pytest.fixture
def start_worker() -> Iterator[Callable[..., Worker]]:
    """Start ``python -m rag_worker`` with a given REDIS_URL; always killed at teardown.

    Extra positional arguments are passwords to mask in failure messages.
    """
    started: list[Worker] = []

    def start(redis_url: str, *secrets: str) -> Worker:
        worker = Worker(redis_url, *secrets)
        started.append(worker)
        return worker

    yield start
    for worker in started:
        worker.close()

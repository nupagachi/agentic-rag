"""JSON logging (structlog). One JSON object per line with ``ts``, ``level``, ``msg``, ``stage``.

Named ``logging`` on purpose (task scope); the stdlib module is still reachable through
absolute imports.
"""

from __future__ import annotations

import logging
import sys
from typing import TextIO

import structlog
from structlog.processors import ExceptionRenderer
from structlog.tracebacks import ExceptionDictTransformer

_LEVELS = {
    "debug": logging.DEBUG,
    "info": logging.INFO,
    "warning": logging.WARNING,
    "error": logging.ERROR,
}


def configure_logging(level: str, stage: str, *, stream: TextIO | None = None) -> None:
    """Configure structlog for JSON output on ``stream`` (default stdout).

    Tracebacks are rendered without local variables (SR-7).
    """
    structlog.contextvars.clear_contextvars()
    structlog.contextvars.bind_contextvars(stage=stage)
    structlog.configure(
        processors=[
            structlog.contextvars.merge_contextvars,
            structlog.processors.add_log_level,
            structlog.processors.TimeStamper(fmt="iso", utc=True, key="ts"),
            ExceptionRenderer(ExceptionDictTransformer(show_locals=False)),
            structlog.processors.EventRenamer("msg"),
            structlog.processors.JSONRenderer(),
        ],
        wrapper_class=structlog.make_filtering_bound_logger(_LEVELS.get(level, logging.INFO)),
        logger_factory=structlog.WriteLoggerFactory(file=stream or sys.stdout),
        cache_logger_on_first_use=False,
    )


def get_logger() -> structlog.typing.FilteringBoundLogger:
    """Return the worker logger (configure with :func:`configure_logging` first)."""
    return structlog.get_logger()  # type: ignore[no-any-return]

"""Entry point: ``python -m rag_worker [--stage NAME]``."""

from __future__ import annotations

import argparse
import asyncio
import re
import sys
from collections.abc import Sequence

from pydantic import ValidationError

from rag_worker.app import EXIT_ERROR, EXIT_OK, run
from rag_worker.config import describe_errors, load_settings
from rag_worker.logging import configure_logging, get_logger

DEFAULT_STAGE = "idle"
_STAGE_PATTERN = r"[a-z][a-z0-9_-]{0,31}"


def _stage(value: str) -> str:
    if not re.fullmatch(_STAGE_PATTERN, value):
        raise argparse.ArgumentTypeError(f"must match {_STAGE_PATTERN}")
    return value


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="rag_worker", description="Agentic RAG worker")
    parser.add_argument(
        "--stage",
        type=_stage,
        default=DEFAULT_STAGE,
        help=f"stage name, only recorded in logs (default: {DEFAULT_STAGE})",
    )
    args = parser.parse_args(argv)

    configure_logging("info", args.stage)  # until the configured level is known
    try:
        settings = load_settings()
    except ValidationError as exc:
        # Variable names and error types only; never the offending values (SR-7).
        get_logger().error("config_invalid", errors=describe_errors(exc))
        return EXIT_ERROR

    configure_logging(settings.log_level, args.stage)
    try:
        return asyncio.run(run(settings))
    except KeyboardInterrupt:
        return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())

"""Điểm vào: python -m rag_worker --stage parse"""

import argparse
import signal
import sys
import threading
from typing import get_args

from rag_worker.platform import cache, config, database, logger


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="rag_worker")
    parser.add_argument("--stage", required=True, choices=get_args(config.Stage))
    args = parser.parse_args(argv)

    cfg = config.load()
    log = logger.setup(cfg)

    rdb = cache.new(cfg)
    pool = database.new(cfg)

    stop = threading.Event()
    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, lambda *_: stop.set())

    log.info("worker started", extra={"stage": args.stage, "env": cfg.app_env})
    try:
        # TODO(M0-04): consumer Redis Streams cho stage
        stop.wait()
    finally:
        log.info("worker stopping", extra={"stage": args.stage})
        pool.close()
        rdb.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())

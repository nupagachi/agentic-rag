"""Logger dùng chung: JSON ra stdout (mặc định) hoặc text khi dev."""

import json
import logging
import sys
from datetime import UTC, datetime

from rag_worker.platform.config import Config

_RESERVED = set(logging.makeLogRecord({}).__dict__) | {"message", "asctime"}


class JSONFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        entry = {
            "time": datetime.fromtimestamp(record.created, UTC).isoformat(timespec="milliseconds"),
            "level": record.levelname.lower(),
            "logger": record.name,
            "message": record.getMessage(),
        }
        # Trường thêm qua extra={...}
        entry.update({k: v for k, v in record.__dict__.items() if k not in _RESERVED})
        if record.exc_info:
            entry["error"] = self.formatException(record.exc_info)
        return json.dumps(entry, ensure_ascii=False, default=str)


def setup(cfg: Config) -> logging.Logger:
    handler = logging.StreamHandler(sys.stdout)
    if cfg.log.format == "text":
        handler.setFormatter(logging.Formatter("%(asctime)s %(levelname)s %(name)s: %(message)s"))
    else:
        handler.setFormatter(JSONFormatter())

    root = logging.getLogger()
    root.handlers[:] = [handler]
    root.setLevel(cfg.log.level.upper())
    return logging.getLogger("rag_worker")

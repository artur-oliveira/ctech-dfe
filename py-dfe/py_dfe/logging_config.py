"""Structured JSON logging for CloudWatch Logs Insights."""
from __future__ import annotations

import json
import logging
import traceback
from datetime import datetime, timezone

# Third-party loggers that are too verbose at INFO level in Lambda
_QUIET_LOGGERS = (
    "httpx", "httpcore", "urllib3", "botocore",
    "boto3", "s3transfer", "urllib3.connectionpool",
)


class _JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        entry: dict = {
            "timestamp": datetime.fromtimestamp(record.created, tz=timezone.utc).isoformat(),
            "level": record.levelname,
            "logger": record.name,
            "message": record.getMessage(),
        }
        if record.exc_info:
            entry["exception"] = traceback.format_exception(*record.exc_info)
        return json.dumps(entry, ensure_ascii=False, default=str)


def configure() -> None:
    root = logging.getLogger()
    if any(isinstance(h.formatter, _JsonFormatter) for h in root.handlers):
        return

    formatter = _JsonFormatter()
    if root.handlers:
        for h in root.handlers:
            h.setFormatter(formatter)
    else:
        handler = logging.StreamHandler()
        handler.setFormatter(formatter)
        root.addHandler(handler)

    root.setLevel(logging.INFO)

    # Route Python warnings (e.g. DeprecationWarning) through logging → JSON
    logging.captureWarnings(True)

    # Suppress chatty third-party INFO/DEBUG logs
    for name in _QUIET_LOGGERS:
        logging.getLogger(name).setLevel(logging.WARNING)

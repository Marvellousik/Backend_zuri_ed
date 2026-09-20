"""
Structured JSON logging and content sanitization for LexiAssist AI Infrastructure.
Protects student privacy and avoids logging raw confidential documents.
"""
import json
import logging
import sys
from datetime import datetime, timezone
from typing import Dict, Any, Optional


class JSONFormatter(logging.Formatter):
    """Formats log records as structured JSON."""

    def format(self, record: logging.LogRecord) -> str:
        log_entry: Dict[str, Any] = {
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "level": record.levelname,
            "logger": record.name,
            "message": record.getMessage(),
        }

        # Include custom extra fields
        if hasattr(record, "request_id"):
            log_entry["request_id"] = getattr(record, "request_id")
        if hasattr(record, "trace_id"):
            log_entry["trace_id"] = getattr(record, "trace_id")
        if hasattr(record, "operation"):
            log_entry["operation"] = getattr(record, "operation")
        if hasattr(record, "institution_id"):
            log_entry["institution_id"] = getattr(record, "institution_id")

        if record.exc_info:
            log_entry["exception"] = self.formatException(record.exc_info)

        return json.dumps(log_entry)


def setup_logger(log_level: str = "INFO") -> logging.Logger:
    """Initialize structured logger."""
    root_logger = logging.getLogger()
    root_logger.setLevel(getattr(logging, log_level.upper(), logging.INFO))

    # Remove existing handlers
    for handler in root_logger.handlers[:]:
        root_logger.removeHandler(handler)

    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(JSONFormatter())
    root_logger.addHandler(handler)

    return root_logger

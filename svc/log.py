import logging
import json
import time
from typing import Any


class JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        log_record = {
            "message": record.getMessage(),
            # Defaults to current time in milliseconds if not set
            "timestamp": record.__dict__.get("timestamp", int(time.time() * 1000)),
            # Defaults to empty dict if not set
            "tags": record.__dict__.get("tags", {}),
        }
        return json.dumps(log_record)


class ContextFilter(logging.Filter):
    def __init__(self) -> None:
        super().__init__()
        self.context = {}

    def set_context(self, **kwargs: Any) -> None:
        self.context.update(kwargs)

    def filter(self, record: logging.LogRecord) -> bool:
        record.tags = self.context
        return True


# Set up logger
logger = logging.getLogger()
logger.setLevel(logging.INFO)  # Set the default logging level

console_handler = logging.StreamHandler()
console_handler.setLevel(logging.INFO)  # Set the logging level for the handler

context_filter = ContextFilter()  # Set filter
console_handler.addFilter(context_filter)
logger.addFilter(context_filter)

json_formatter = JsonFormatter()  # Set formatter
console_handler.setFormatter(json_formatter)

logger.addHandler(console_handler)  # Add handler to logger

"""
Cancellation and Token Management for LexiAssist AI Infrastructure.
Allows client disconnects or explicit stop requests to terminate active processing.
"""
import asyncio
import logging
from typing import Dict, Optional

logger = logging.getLogger(__name__)


class CancellationToken:
    """Represents a cancellation signal for an ongoing request."""

    def __init__(self, request_id: str):
        self.request_id = request_id
        self._is_cancelled = False
        self._callbacks = []

    @property
    def is_cancelled(self) -> bool:
        return self._is_cancelled

    def cancel(self):
        self._is_cancelled = True
        for callback in self._callbacks:
            try:
                if asyncio.iscoroutinefunction(callback):
                    asyncio.create_task(callback())
                else:
                    callback()
            except Exception as e:
                logger.error(f"Error in cancellation callback: {e}")

    def on_cancelled(self, callback):
        self._callbacks.append(callback)


class CancellationManager:
    """Tracks active cancellation tokens by request ID."""

    def __init__(self):
        self._tokens: Dict[str, CancellationToken] = {}

    def get_or_create(self, request_id: str) -> CancellationToken:
        if request_id not in self._tokens:
            self._tokens[request_id] = CancellationToken(request_id)
        return self._tokens[request_id]

    def cancel(self, request_id: str) -> bool:
        if request_id in self._tokens:
            self._tokens[request_id].cancel()
            logger.info(f"🛑 Cancelled request {request_id}")
            return True
        return False

    def remove(self, request_id: str):
        self._tokens.pop(request_id, None)

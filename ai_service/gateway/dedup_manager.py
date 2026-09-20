"""
In-Flight Request Deduplication Manager for LexiAssist AI Infrastructure.
Collapses multiple concurrent identical requests into a single provider execution.
"""
import json
import asyncio
import hashlib
import logging
from typing import Optional, Dict, Any, Tuple
from ai_service.storage.redis_client import get_redis_client

logger = logging.getLogger(__name__)


class DedupManager:
    """Manages collapsing of in-flight identical workloads."""

    def __init__(self):
        self.redis = get_redis_client()
        self._local_in_flight: Dict[str, asyncio.Future] = {}

    @staticmethod
    def compute_dedup_key(operation: str, payload_data: Dict[str, Any]) -> str:
        serialized = json.dumps(payload_data, sort_keys=True, default=str)
        payload_hash = hashlib.sha256(serialized.encode()).hexdigest()
        return f"ai_dedup:{operation}:{payload_hash}"

    async def acquire_or_wait(
        self,
        dedup_key: str,
        timeout_seconds: float = 30.0,
    ) -> Tuple[bool, Optional[Dict[str, Any]]]:
        """
        Attempt to claim primary execution role for this workload.
        Returns: (is_primary_executor, shared_result_if_secondary)
        """
        # Check local process in-flight map first
        if dedup_key in self._local_in_flight:
            logger.info(f"👥 Request deduplicated locally (waiting for primary): {dedup_key}")
            try:
                future = self._local_in_flight[dedup_key]
                result = await asyncio.wait_for(asyncio.shield(future), timeout=timeout_seconds)
                return False, result
            except asyncio.TimeoutError:
                logger.warning(f"Dedup wait timed out for {dedup_key}, falling back to execution")
                return True, None
            except Exception as e:
                logger.warning(f"Error waiting on dedup future: {e}")
                return True, None

        # Create new in-flight future for this process
        loop = asyncio.get_event_loop()
        future = loop.create_future()
        self._local_in_flight[dedup_key] = future
        return True, None

    async def complete_and_broadcast(self, dedup_key: str, result: Optional[Dict[str, Any]]):
        """Notify any waiting coroutines of the execution result."""
        if dedup_key in self._local_in_flight:
            future = self._local_in_flight.pop(dedup_key)
            if not future.done():
                if result is not None:
                    future.set_result(result)
                else:
                    future.set_result({})

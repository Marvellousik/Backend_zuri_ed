"""
Retry Policy Manager for LexiAssist AI Infrastructure.
Implements exponential backoff with full jitter respecting request deadlines.
"""
import time
import random
import asyncio
import logging
from typing import Callable, Any, Optional

logger = logging.getLogger(__name__)


class RetryManager:
    """Executes retryable async functions with jittered exponential backoff."""

    @staticmethod
    async def execute_with_retry(
        func: Callable[[], Any],
        max_retries: int = 2,
        base_delay_seconds: float = 1.0,
        deadline_timestamp: Optional[float] = None,
    ) -> Any:
        """
        Execute func with exponential backoff and jitter up to max_retries.
        Respects deadline_timestamp if provided.
        """
        last_exception = None
        
        for attempt in range(max_retries + 1):
            try:
                # Check if deadline expired before executing
                if deadline_timestamp and time.time() >= deadline_timestamp:
                    raise TimeoutError("Request deadline budget exhausted before retry attempt")

                return await func()

            except Exception as e:
                last_exception = e
                if attempt == max_retries:
                    break

                # Calculate jittered exponential backoff: delay = random(0, base * 2^attempt)
                backoff = min(10.0, base_delay_seconds * (2 ** attempt))
                jittered_delay = random.uniform(0.5 * backoff, backoff)

                # Check if sleeping would exceed deadline
                if deadline_timestamp and (time.time() + jittered_delay >= deadline_timestamp):
                    logger.warning("Remaining deadline budget insufficient for next retry")
                    break

                logger.info(f"Retrying in {jittered_delay:.2f}s (attempt {attempt + 1}/{max_retries}) due to: {e}")
                await asyncio.sleep(jittered_delay)

        raise last_exception or RuntimeError("Operation failed with unknown error")

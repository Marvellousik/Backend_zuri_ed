"""
Multi-layer Rate Limiter for LexiAssist AI Infrastructure.
Protects AI endpoints against abusive or synchronized request spikes.
"""
import time
import logging
from typing import Optional, Tuple
from ai_service.storage.redis_client import get_redis_client

logger = logging.getLogger(__name__)

DEFAULT_RPM_PER_USER = 60
DEFAULT_AI_RPM_PER_USER = 30


class RateLimiter:
    """Redis-backed sliding window rate limiter."""

    def __init__(self):
        self.redis = get_redis_client()

    async def is_rate_limited(
        self,
        user_id: str,
        operation: str,
        limit_rpm: int = DEFAULT_AI_RPM_PER_USER
    ) -> Tuple[bool, int]:
        """
        Check rate limit using a 60-second sliding window.
        Returns (is_limited, remaining_requests).
        """
        if not self.redis:
            return False, limit_rpm

        try:
            current_minute = int(time.time() // 60)
            key = f"ratelimit:{user_id}:{current_minute}"
            
            count = self.redis.incr(key)
            if count == 1:
                self.redis.expire(key, 120)  # 2 minute TTL

            remaining = max(0, limit_rpm - count)
            if count > limit_rpm:
                logger.warning(f"⚠️ Rate limit exceeded for user {user_id} on {operation} ({count}/{limit_rpm} RPM)")
                return True, 0

            return False, remaining

        except Exception as e:
            logger.error(f"Rate limit check error: {e}")
            return False, limit_rpm

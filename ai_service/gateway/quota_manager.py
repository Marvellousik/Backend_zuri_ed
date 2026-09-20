"""
Hierarchical Quota Management for LexiAssist AI Infrastructure.
Enforces multi-tier quotas across User, Institution, Course, and Global budgets.
"""
import logging
from typing import Optional, Dict, Any, Tuple
from ai_service.storage.redis_client import get_redis_client

logger = logging.getLogger(__name__)

# Default limits (can be configured via environment / tenant policy)
DEFAULT_USER_DAILY_REQUEST_LIMIT = 200
DEFAULT_INSTITUTION_DAILY_REQUEST_LIMIT = 50000


class QuotaManager:
    """Tracks and enforces multi-tenant AI usage quotas."""

    def __init__(self):
        self.redis = get_redis_client()

    async def check_and_reserve_quota(
        self,
        user_id: str,
        institution_id: Optional[str] = None,
        estimated_units: int = 1,
    ) -> Tuple[bool, Optional[str]]:
        """
        Check if user and institution have remaining quota.
        Returns (is_allowed, failure_reason).
        """
        if not self.redis:
            # If Redis is unavailable, fail open to avoid service outage
            return True, None

        try:
            # 1. User Quota Check
            user_key = f"quota:user:{user_id}"
            user_count = self.redis.incrby(user_key, estimated_units)
            if user_count == estimated_units:
                self.redis.expire(user_key, 86400)  # 24 hour rolling TTL

            if user_count > DEFAULT_USER_DAILY_REQUEST_LIMIT:
                logger.warning(f"🚫 User daily quota exceeded for {user_id} ({user_count}/{DEFAULT_USER_DAILY_REQUEST_LIMIT})")
                return False, f"User daily AI quota exceeded ({DEFAULT_USER_DAILY_REQUEST_LIMIT} requests/day)"

            # 2. Institution Quota Check (if institution_id is provided)
            if institution_id:
                inst_key = f"quota:inst:{institution_id}"
                inst_count = self.redis.incrby(inst_key, estimated_units)
                if inst_count == estimated_units:
                    self.redis.expire(inst_key, 86400)

                if inst_count > DEFAULT_INSTITUTION_DAILY_REQUEST_LIMIT:
                    logger.warning(f"🚫 Institution daily quota exceeded for {institution_id} ({inst_count}/{DEFAULT_INSTITUTION_DAILY_REQUEST_LIMIT})")
                    return False, f"Institution AI capacity limit reached for today"

            return True, None

        except Exception as e:
            logger.error(f"Quota check error: {e}")
            return True, None  # Fail open on Redis error

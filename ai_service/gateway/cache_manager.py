"""
Multi-Tenant Scoped Caching Manager for LexiAssist AI Infrastructure.
Ensures deterministic cache lookup with security boundaries (GLOBAL, INSTITUTION, COURSE, USER).
"""
import json
import hashlib
import logging
from typing import Optional, Dict, Any, Tuple
from enum import Enum

from ai_service.storage.redis_client import get_redis_client

logger = logging.getLogger(__name__)


class CacheScope(str, Enum):
    GLOBAL = "GLOBAL"           # Shareable across all users/institutions (definitions, general facts)
    INSTITUTION = "INSTITUTION" # Shareable within a single university/school
    COURSE = "COURSE"           # Shareable within a course / subject (CSC301 summaries)
    DOCUMENT = "DOCUMENT"       # Scoped to a specific document ID
    USER = "USER"               # Scoped privately to one user (private essays, personal notes)
    SESSION = "SESSION"         # Scoped to a single interactive conversation


class CacheManager:
    """Manages AI response caching across tenant boundaries."""

    def __init__(self):
        self.redis = get_redis_client()

    @staticmethod
    def compute_cache_key(
        scope: CacheScope,
        scope_id: str,
        operation: str,
        operation_version: str,
        payload_data: Dict[str, Any],
        tool_version: str = "1.0",
    ) -> str:
        """
        Compute deterministic SHA-256 cache key.
        """
        serialized_payload = json.dumps(payload_data, sort_keys=True, default=str)
        payload_hash = hashlib.sha256(serialized_payload.encode()).hexdigest()
        
        raw_key = f"{scope.value}:{scope_id}:{operation}:{operation_version}:{tool_version}:{payload_hash}"
        return f"ai_cache:{hashlib.sha256(raw_key.encode()).hexdigest()}"

    async def get_cached_result(
        self,
        scope: CacheScope,
        scope_id: str,
        operation: str,
        operation_version: str,
        payload_data: Dict[str, Any],
    ) -> Optional[Dict[str, Any]]:
        """Lookup cached AI result in Redis."""
        if not self.redis:
            return None

        try:
            cache_key = self.compute_cache_key(scope, scope_id, operation, operation_version, payload_data)
            cached_data = self.redis.get(cache_key)
            if cached_data:
                logger.info(f"♻️ Cache HIT [{scope.value}:{operation}]")
                return json.loads(cached_data)
            return None
        except Exception as e:
            logger.warning(f"Cache read error: {e}")
            return None

    async def set_cached_result(
        self,
        scope: CacheScope,
        scope_id: str,
        operation: str,
        operation_version: str,
        payload_data: Dict[str, Any],
        result: Dict[str, Any],
        ttl_seconds: int = 86400,  # 24 hours default
    ) -> bool:
        """Store AI result in Redis cache."""
        if not self.redis or not result:
            return False

        try:
            cache_key = self.compute_cache_key(scope, scope_id, operation, operation_version, payload_data)
            self.redis.setex(cache_key, ttl_seconds, json.dumps(result, default=str))
            return True
        except Exception as e:
            logger.warning(f"Cache write error: {e}")
            return False

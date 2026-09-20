"""
Redis client wrapper with connection pooling, retries, and helper utilities.
"""
import os
import logging
from typing import Optional, Any
try:
    import redis
except ImportError:
    redis = None

logger = logging.getLogger(__name__)

REDIS_URL = os.getenv("REDIS_URL", "redis://localhost:6379")

_redis_client: Optional[Any] = None


def get_redis_client() -> Optional[Any]:
    """Get or lazily initialize the singleton Redis client."""
    global _redis_client
    if redis is None:
        return None
    if _redis_client is None:
        try:
            _redis_client = redis.from_url(
                REDIS_URL,
                decode_responses=True,
                socket_timeout=5.0,
                socket_connect_timeout=5.0,
                retry_on_timeout=True,
                health_check_interval=30,
            )
            # Verify connectivity
            _redis_client.ping()
            logger.info("✅ Connected to Redis successfully")
        except Exception as e:
            logger.warning(f"⚠️ Redis connection failed ({REDIS_URL}): {e}. Features requiring Redis will gracefully degrade.")
            _redis_client = None
    return _redis_client


def is_redis_available() -> bool:
    """Check if Redis connection is currently active."""
    r = get_redis_client()
    if r is None:
        return False
    try:
        return bool(r.ping())
    except Exception:
        return False

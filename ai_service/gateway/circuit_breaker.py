"""
Circuit breaker for protecting external AI providers from cascading failures.
Implements CLOSED -> OPEN -> HALF_OPEN state machine.
"""
import time
import logging
from enum import Enum
from typing import Dict, Any

logger = logging.getLogger(__name__)


class CircuitState(str, Enum):
    CLOSED = "CLOSED"       # Normal operation, requests allowed
    OPEN = "OPEN"           # Tripped, requests blocked immediately
    HALF_OPEN = "HALF_OPEN" # Cooldown expired, testing with probe request


class CircuitBreaker:
    """Per-provider circuit breaker."""

    def __init__(self, failure_threshold: int = 5, recovery_timeout_seconds: float = 30.0):
        self.failure_threshold = failure_threshold
        self.recovery_timeout_seconds = recovery_timeout_seconds
        
        self.state = CircuitState.CLOSED
        self.failure_count = 0
        self.last_failure_time = 0.0
        self.success_count = 0

    def allow_request(self) -> bool:
        """Check if request can proceed through circuit breaker."""
        now = time.time()
        
        if self.state == CircuitState.OPEN:
            if now - self.last_failure_time > self.recovery_timeout_seconds:
                logger.info("🔄 Circuit breaker transitioned to HALF_OPEN (probing health)")
                self.state = CircuitState.HALF_OPEN
                self.success_count = 0
                return True
            return False
            
        return True

    def record_success(self):
        """Record successful execution."""
        if self.state == CircuitState.HALF_OPEN:
            self.success_count += 1
            if self.success_count >= 2:
                logger.info("✅ Circuit breaker recovered to CLOSED state")
                self.state = CircuitState.CLOSED
                self.failure_count = 0
        elif self.state == CircuitState.CLOSED:
            self.failure_count = 0

    def record_failure(self):
        """Record failed execution and trip circuit if threshold exceeded."""
        self.last_failure_time = time.time()
        self.failure_count += 1
        
        if self.state in (CircuitState.CLOSED, CircuitState.HALF_OPEN) and self.failure_count >= self.failure_threshold:
            logger.error(f"🚨 Circuit breaker TRIPPED to OPEN ({self.failure_count} consecutive failures)")
            self.state = CircuitState.OPEN

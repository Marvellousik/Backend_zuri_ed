"""
Timeout and Deadline Management for LexiAssist AI Infrastructure.
Provides execution deadlines based on TimeoutClass.
"""
import time
from typing import Dict
from ai_service.contracts.context import TimeoutClass

TIMEOUT_LIMITS_SECONDS: Dict[TimeoutClass, float] = {
    TimeoutClass.INTERACTIVE_FAST: 10.0,
    TimeoutClass.INTERACTIVE_STREAM: 45.0,
    TimeoutClass.STANDARD: 35.0,
    TimeoutClass.LONG_RUNNING: 300.0,
    TimeoutClass.BACKGROUND: 600.0,
}


class TimeoutManager:
    """Calculates and manages remaining request deadline budgets."""

    @staticmethod
    def get_deadline_seconds(timeout_class: TimeoutClass) -> float:
        return TIMEOUT_LIMITS_SECONDS.get(timeout_class, 30.0)

    @staticmethod
    def create_deadline(timeout_class: TimeoutClass) -> float:
        """Return absolute timestamp when request deadline expires."""
        return time.time() + TimeoutManager.get_deadline_seconds(timeout_class)

    @staticmethod
    def remaining_seconds(deadline_timestamp: float) -> float:
        """Get remaining seconds before deadline."""
        remaining = deadline_timestamp - time.time()
        return max(0.0, remaining)

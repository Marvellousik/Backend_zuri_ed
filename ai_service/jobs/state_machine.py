"""
Job State Machine for LexiAssist AI Infrastructure.
Enforces valid lifecycle transitions and terminal states.
"""
from enum import Enum
from typing import Set


class JobStatus(str, Enum):
    CREATED = "created"
    QUEUED = "queued"
    CLAIMED = "claimed"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"
    DROPPED = "dropped"
    EXPIRED = "expired"


# Valid state transitions
VALID_TRANSITIONS = {
    JobStatus.CREATED: {JobStatus.QUEUED, JobStatus.DROPPED, JobStatus.REJECTED if hasattr(JobStatus, 'REJECTED') else JobStatus.FAILED},
    JobStatus.QUEUED: {JobStatus.CLAIMED, JobStatus.CANCELLED, JobStatus.DROPPED, JobStatus.EXPIRED},
    JobStatus.CLAIMED: {JobStatus.RUNNING, JobStatus.FAILED, JobStatus.CANCELLED},
    JobStatus.RUNNING: {JobStatus.COMPLETED, JobStatus.FAILED, JobStatus.CANCELLED},
    JobStatus.COMPLETED: set(),
    JobStatus.FAILED: {JobStatus.QUEUED},  # Requeue on retry
    JobStatus.CANCELLED: set(),
    JobStatus.DROPPED: set(),
    JobStatus.EXPIRED: set(),
}


def can_transition(current_status: JobStatus, target_status: JobStatus) -> bool:
    """Check if state transition is allowed."""
    allowed = VALID_TRANSITIONS.get(current_status, set())
    return target_status in allowed

"""
Unit tests for Job State Machine and Queue Managers.
"""
from ai_service.jobs.state_machine import JobStatus, can_transition


def test_job_state_machine_valid_transitions():
    assert can_transition(JobStatus.QUEUED, JobStatus.CLAIMED) is True
    assert can_transition(JobStatus.CLAIMED, JobStatus.RUNNING) is True
    assert can_transition(JobStatus.RUNNING, JobStatus.COMPLETED) is True
    assert can_transition(JobStatus.RUNNING, JobStatus.FAILED) is True


def test_job_state_machine_invalid_transitions():
    assert can_transition(JobStatus.COMPLETED, JobStatus.RUNNING) is False
    assert can_transition(JobStatus.CANCELLED, JobStatus.QUEUED) is False

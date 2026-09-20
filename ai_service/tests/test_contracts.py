"""
Unit tests for AI Service Contracts and Envelopes.
"""
import pytest
from ai_service.contracts.context import (
    AIRequestContext,
    PrincipalContext,
    TenantContext,
    CourseContext,
    UserRole,
    ExecutionPriority,
    TimeoutClass,
)
from ai_service.contracts.execution import (
    AIExecutionEnvelope,
    ExecutionState,
    ExecutionMode,
)
from ai_service.contracts.errors import FailureCode, FailureStage, AIErrorDetail
from ai_service.contracts.usage import TokenUsage, TimingMetrics


def test_ai_request_context_creation():
    ctx = AIRequestContext(
        request_id="req_test_123",
        trace_id="trace_test_123",
        principal=PrincipalContext(user_id="user_1", role=UserRole.STUDENT),
        tenant=TenantContext(institution_id="uni_lagos"),
        course=CourseContext(course_id="CSC301"),
        operation="study.flashcards.generate",
        input={"text": "Recursion is when a function calls itself."},
        parameters={"count": 5},
    )

    assert ctx.request_id == "req_test_123"
    assert ctx.principal.user_id == "user_1"
    assert ctx.tenant.institution_id == "uni_lagos"
    assert ctx.course.course_id == "CSC301"
    assert ctx.operation == "study.flashcards.generate"
    assert ctx.policy.priority == ExecutionPriority.NORMAL
    assert ctx.policy.timeout_class == TimeoutClass.STANDARD


def test_ai_request_context_optional_institution():
    """Verify that institution_id is fully optional and defaults to None for B2C users."""
    ctx = AIRequestContext(
        request_id="req_test_456",
        trace_id="trace_test_456",
        principal=PrincipalContext(user_id="user_b2c"),
        operation="chat.generate",
        input={"query": "Explain binary search."},
    )

    assert ctx.tenant.institution_id is None
    assert ctx.course.course_id is None
    assert ctx.principal.user_id == "user_b2c"


def test_ai_execution_envelope_success():
    envelope = AIExecutionEnvelope(
        request_id="req_test_123",
        trace_id="trace_test_123",
        status=ExecutionState.COMPLETED,
        result={"flashcards": [{"front": "Q", "back": "A"}]},
        usage=TokenUsage(input_tokens=100, output_tokens=50, total_tokens=150),
    )

    assert envelope.is_success() is True
    assert envelope.status == ExecutionState.COMPLETED
    assert envelope.result["flashcards"][0]["front"] == "Q"
    assert envelope.usage.total_tokens == 150


def test_ai_execution_envelope_error():
    envelope = AIExecutionEnvelope(
        request_id="req_test_123",
        trace_id="trace_test_123",
        status=ExecutionState.FAILED,
        error=AIErrorDetail(
            code=FailureCode.PROVIDER_TIMEOUT,
            stage=FailureStage.MODEL_EXECUTION,
            message="Gemini provider timed out after 30s",
            retryable=True,
        ),
    )

    assert envelope.is_success() is False
    assert envelope.error.code == FailureCode.PROVIDER_TIMEOUT
    assert envelope.error.retryable is True

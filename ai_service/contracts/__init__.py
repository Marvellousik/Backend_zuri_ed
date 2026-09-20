"""
AI Contracts package for LexiAssist AI Infrastructure.
"""
from ai_service.contracts.context import (
    AIRequestContext,
    PrincipalContext,
    TenantContext,
    CourseContext,
    ClientContext,
    PolicyContext,
    UserRole,
    ExecutionPriority,
    TimeoutClass,
)
from ai_service.contracts.execution import (
    AIExecutionEnvelope,
    ExecutionMode,
    ExecutionState,
    ExecutionMetadata,
    SourceInfo,
)
from ai_service.contracts.errors import (
    FailureCode,
    FailureStage,
    AIErrorDetail,
)
from ai_service.contracts.usage import (
    TokenUsage,
    TimingMetrics,
    AIUsageRecord,
    AIRequestAudit,
)
from ai_service.contracts.events import (
    StreamEventType,
    StreamStartedEvent,
    StreamThinkingEvent,
    StreamTokenEvent,
    StreamCompletedEvent,
    StreamErrorEvent,
)

__all__ = [
    "AIRequestContext",
    "PrincipalContext",
    "TenantContext",
    "CourseContext",
    "ClientContext",
    "PolicyContext",
    "UserRole",
    "ExecutionPriority",
    "TimeoutClass",
    "AIExecutionEnvelope",
    "ExecutionMode",
    "ExecutionState",
    "ExecutionMetadata",
    "SourceInfo",
    "FailureCode",
    "FailureStage",
    "AIErrorDetail",
    "TokenUsage",
    "TimingMetrics",
    "AIUsageRecord",
    "AIRequestAudit",
    "StreamEventType",
    "StreamStartedEvent",
    "StreamThinkingEvent",
    "StreamTokenEvent",
    "StreamCompletedEvent",
    "StreamErrorEvent",
]

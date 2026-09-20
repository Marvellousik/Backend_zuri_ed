"""
Server-Sent Events (SSE) streaming event schemas for LexiAssist AI Infrastructure.
Defines typed event payloads for real-time streaming operations.
"""
from enum import Enum
from typing import Optional, Dict, Any, List
from pydantic import BaseModel, Field

from ai_service.contracts.usage import TokenUsage
from ai_service.contracts.errors import FailureCode


class StreamEventType(str, Enum):
    STARTED = "started"
    THINKING = "thinking"  # Execution state indicator, not raw internal chain-of-thought
    TOKEN = "token"
    COMPLETED = "completed"
    ERROR = "error"


class StreamStartedEvent(BaseModel):
    request_id: str
    trace_id: str
    session_id: Optional[str] = None
    operation: str


class StreamThinkingEvent(BaseModel):
    """Signals that the AI Gateway or tool is processing a sub-step."""
    stage: str = Field(..., description="e.g. 'retrieving_context', 'analyzing_document'")
    message: Optional[str] = None


class StreamTokenEvent(BaseModel):
    token: str
    index: int = 0


class StreamCompletedEvent(BaseModel):
    request_id: str
    trace_id: str
    status: str = "completed"
    usage: TokenUsage
    duration_ms: int
    sources: List[str] = Field(default_factory=list)


class StreamErrorEvent(BaseModel):
    request_id: str
    trace_id: str
    status: str = "failed"
    failure_code: FailureCode
    message: str
    retryable: bool = False

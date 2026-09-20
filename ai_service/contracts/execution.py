"""
Execution and response envelope schemas for LexiAssist AI Infrastructure.
Defines execution lifecycle states, response envelopes, and result models.
"""
from enum import Enum
from typing import Optional, Dict, Any, List
from datetime import datetime, timezone
from pydantic import BaseModel, Field

from ai_service.contracts.usage import TokenUsage, TimingMetrics, AIUsageRecord
from ai_service.contracts.errors import AIErrorDetail


class ExecutionMode(str, Enum):
    SYNC = "sync"
    STREAM = "stream"
    ASYNC_JOB = "async_job"
    BACKGROUND = "background"


class ExecutionState(str, Enum):
    """Lifecycle states through the AI Gateway execution pipeline."""
    RECEIVED = "RECEIVED"
    VALIDATING = "VALIDATING"
    AUTHORIZED = "AUTHORIZED"
    CLASSIFIED = "CLASSIFIED"
    CACHE_CHECK = "CACHE_CHECK"
    QUEUED = "QUEUED"
    EXECUTING = "EXECUTING"
    RETRIEVING = "RETRIEVING"
    TOOL_EXECUTION = "TOOL_EXECUTION"
    MODEL_EXECUTION = "MODEL_EXECUTION"
    VALIDATING_RESPONSE = "VALIDATING_RESPONSE"
    COMPLETED = "COMPLETED"
    
    # Terminal alternate states
    FAILED = "FAILED"
    DROPPED = "DROPPED"
    REJECTED = "REJECTED"
    CANCELLED = "CANCELLED"
    EXPIRED = "EXPIRED"


class SourceInfo(BaseModel):
    """Information on whether the result was freshly generated, cached, or deduplicated."""
    type: str = Field(default="model", description="model, cache, or deduplication")
    cached: bool = Field(default=False, description="True if served from cache")
    cache_scope: Optional[str] = Field(default=None, description="Scope of cache hit (GLOBAL, INSTITUTION, COURSE, USER)")
    deduplicated: bool = Field(default=False, description="True if collapsed into concurrent in-flight execution")
    provider: Optional[str] = Field(default=None, description="Provider used (e.g. google, cohere)")
    model: Optional[str] = Field(default=None, description="Model used (e.g. gemini-2.5-flash)")


class ExecutionMetadata(BaseModel):
    """Execution timing and state details."""
    mode: ExecutionMode = Field(default=ExecutionMode.SYNC)
    state: ExecutionState = Field(default=ExecutionState.COMPLETED)
    started_at: str = Field(default_factory=lambda: datetime.now(timezone.utc).isoformat())
    completed_at: Optional[str] = Field(default=None)
    duration_ms: Optional[int] = Field(default=None)
    timings: Optional[TimingMetrics] = Field(default=None)


class AIExecutionEnvelope(BaseModel):
    """
    Standard AI response envelope returned to Go Gateway.
    Exposes execution state, validated result, usage, timings, and error info.
    """
    request_id: str = Field(..., description="Unique request ID matching the incoming request")
    trace_id: str = Field(..., description="Distributed trace ID")
    status: ExecutionState = Field(..., description="Terminal execution status")
    
    result: Optional[Dict[str, Any]] = Field(default=None, description="Domain result payload")
    error: Optional[AIErrorDetail] = Field(default=None, description="Error detail if not COMPLETED")
    
    execution: ExecutionMetadata = Field(default_factory=ExecutionMetadata)
    usage: Optional[TokenUsage] = Field(default=None)
    source: SourceInfo = Field(default_factory=SourceInfo)
    
    def is_success(self) -> bool:
        return self.status == ExecutionState.COMPLETED

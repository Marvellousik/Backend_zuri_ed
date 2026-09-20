"""
Usage, telemetry, and audit models for LexiAssist AI Infrastructure.
Tracks multi-level latencies, token consumption, cost attribution, and audit records.
"""
from typing import Optional, Dict, Any
from datetime import datetime, timezone
from pydantic import BaseModel, Field


class TokenUsage(BaseModel):
    """Token accounting model."""
    input_tokens: int = Field(default=0, description="Prompt / input tokens consumed")
    output_tokens: int = Field(default=0, description="Completion / output tokens consumed")
    total_tokens: int = Field(default=0, description="Total tokens consumed")
    audio_seconds: float = Field(default=0.0, description="Audio seconds processed if applicable")
    estimated_cost_usd: float = Field(default=0.0, description="Estimated monetary cost in USD")


class TimingMetrics(BaseModel):
    """Multi-stage latency measurement in milliseconds."""
    queue_ms: int = Field(default=0, description="Time spent waiting in queue before execution")
    retrieval_ms: int = Field(default=0, description="Time spent performing RAG vector/search retrieval")
    tool_ms: int = Field(default=0, description="Time spent executing domain tools")
    ttft_ms: Optional[int] = Field(default=None, description="Time to first token for streaming requests")
    generation_ms: int = Field(default=0, description="Time spent calling model provider")
    validation_ms: int = Field(default=0, description="Time spent validating and parsing output")
    total_duration_ms: int = Field(default=0, description="End-to-end AI service processing latency")


class AIUsageRecord(BaseModel):
    """Normalized usage record for institutional billing and quota tracking."""
    request_id: str
    trace_id: str
    user_id: str
    institution_id: Optional[str] = None
    course_id: Optional[str] = None
    operation: str
    
    provider: str
    model: str
    
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0
    audio_seconds: float = 0.0
    
    estimated_cost_usd: float = 0.0
    
    cache_hit: bool = False
    deduplicated: bool = False
    avoided_cost_usd: float = 0.0
    
    timestamp: str = Field(default_factory=lambda: datetime.now(timezone.utc).isoformat())


class AIRequestAudit(BaseModel):
    """Immutable audit entry for complete request explainability."""
    request_id: str
    trace_id: str
    user_id: str
    institution_id: Optional[str] = None
    course_id: Optional[str] = None
    
    operation: str
    operation_version: str = "1"
    
    status: str
    failure_code: Optional[str] = None
    
    started_at: str
    completed_at: str
    timings: TimingMetrics
    
    cache_hit: bool = False
    cache_scope: Optional[str] = None
    deduplicated: bool = False
    
    provider: Optional[str] = None
    model: Optional[str] = None
    usage: TokenUsage
    
    retry_count: int = 0
    created_at: str = Field(default_factory=lambda: datetime.now(timezone.utc).isoformat())

"""
Failure taxonomy and error models for LexiAssist AI Infrastructure.
Provides structured, machine-readable failure classifications.
"""
from enum import Enum
from typing import Optional, Dict, Any
from datetime import datetime, timezone
from pydantic import BaseModel, Field


class FailureCode(str, Enum):
    # Pre-execution Rejections
    VALIDATION_ERROR = "VALIDATION_ERROR"
    AUTHORIZATION_ERROR = "AUTHORIZATION_ERROR"
    QUOTA_EXCEEDED = "QUOTA_EXCEEDED"
    RATE_LIMITED = "RATE_LIMITED"
    UNSUPPORTED_OPERATION = "UNSUPPORTED_OPERATION"
    POLICY_BLOCKED = "POLICY_BLOCKED"
    CONTENT_POLICY_BLOCKED = "CONTENT_POLICY_BLOCKED"
    
    # Infrastructure Drops (deliberately discarded for capacity protection)
    QUEUE_FULL = "QUEUE_FULL"
    QUEUE_TIMEOUT = "QUEUE_TIMEOUT"
    REQUEST_EXPIRED = "REQUEST_EXPIRED"
    BUDGET_PROTECTION_DROPPED = "BUDGET_PROTECTION_DROPPED"
    SYSTEM_OVERLOAD = "SYSTEM_OVERLOAD"
    
    # Cancellations
    CLIENT_CANCELLED = "CLIENT_CANCELLED"
    CLIENT_DISCONNECTED = "CLIENT_DISCONNECTED"
    ADMIN_CANCELLED = "ADMIN_CANCELLED"
    
    # Execution Failures
    PROVIDER_TIMEOUT = "PROVIDER_TIMEOUT"
    PROVIDER_RATE_LIMIT = "PROVIDER_RATE_LIMIT"
    PROVIDER_UNAVAILABLE = "PROVIDER_UNAVAILABLE"
    PROVIDER_AUTH_ERROR = "PROVIDER_AUTH_ERROR"
    PROVIDER_BAD_RESPONSE = "PROVIDER_BAD_RESPONSE"
    MODEL_OVERLOADED = "MODEL_OVERLOADED"
    TOOL_FAILURE = "TOOL_FAILURE"
    RETRIEVAL_FAILURE = "RETRIEVAL_FAILURE"
    CACHE_ERROR = "CACHE_ERROR"
    STORAGE_FAILURE = "STORAGE_FAILURE"
    INTERNAL_ERROR = "INTERNAL_ERROR"


class FailureStage(str, Enum):
    VALIDATION = "VALIDATION"
    AUTHORIZATION = "AUTHORIZATION"
    CLASSIFICATION = "CLASSIFICATION"
    QUOTA_CHECK = "QUOTA_CHECK"
    RATE_LIMIT_CHECK = "RATE_LIMIT_CHECK"
    CACHE_LOOKUP = "CACHE_LOOKUP"
    QUEUEING = "QUEUEING"
    RETRIEVAL = "RETRIEVAL"
    TOOL_EXECUTION = "TOOL_EXECUTION"
    MODEL_EXECUTION = "MODEL_EXECUTION"
    RESPONSE_VALIDATION = "RESPONSE_VALIDATION"
    STORAGE = "STORAGE"


class AIErrorDetail(BaseModel):
    """
    Standardized error payload returned in AIExecutionEnvelope.
    """
    code: FailureCode = Field(..., description="Machine-readable failure code")
    message: str = Field(..., description="Human-readable safe error message")
    stage: FailureStage = Field(..., description="Pipeline stage where failure occurred")
    retryable: bool = Field(default=False, description="Whether client/gateway can safely retry")
    timestamp: str = Field(default_factory=lambda: datetime.now(timezone.utc).isoformat())
    details: Optional[Dict[str, Any]] = Field(default=None, description="Diagnostic metadata (non-sensitive)")

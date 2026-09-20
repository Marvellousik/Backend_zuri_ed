"""
Request context schemas for LexiAssist AI Infrastructure.
Defines security, tenant, user, course, client, and policy boundaries.
"""
from enum import Enum
from typing import Optional, Dict, Any
from pydantic import BaseModel, Field


class UserRole(str, Enum):
    STUDENT = "student"
    LECTURER = "lecturer"
    ADMIN = "admin"
    GUEST = "guest"
    SYSTEM = "system"


class PrincipalContext(BaseModel):
    """Authenticated user context provided by the upstream gateway."""
    user_id: str = Field(..., description="Unique identifier for the user")
    role: UserRole = Field(default=UserRole.STUDENT, description="User role in the system")
    tier: Optional[str] = Field(default="standard", description="User subscription tier")


class TenantContext(BaseModel):
    """Institutional tenant context for multi-tenancy. Optional for direct/B2C users."""
    institution_id: Optional[str] = Field(default=None, description="University / Institution ID")
    department_id: Optional[str] = Field(default=None, description="Department ID")


class CourseContext(BaseModel):
    """Course and academic context for isolating study materials."""
    course_id: Optional[str] = Field(default=None, description="Course ID or course code")
    semester: Optional[str] = Field(default=None, description="Academic semester or year")


class ClientContext(BaseModel):
    """Client device and application metadata."""
    platform: Optional[str] = Field(default="web", description="Client platform (web, ios, android)")
    app_version: Optional[str] = Field(default="1.0.0", description="Application version")
    client_ip: Optional[str] = Field(default=None, description="Client IP address for telemetry")


class ExecutionPriority(str, Enum):
    CRITICAL = "critical"
    INTERACTIVE = "interactive"
    NORMAL = "normal"
    BACKGROUND = "background"


class TimeoutClass(str, Enum):
    INTERACTIVE_FAST = "interactive_fast"     # e.g., 5-10s
    INTERACTIVE_STREAM = "interactive_stream" # e.g., 3s TTFT, 30s stream
    STANDARD = "standard"                     # e.g., 30s
    LONG_RUNNING = "long_running"             # e.g., async jobs
    BACKGROUND = "background"                 # e.g., queue worker


class PolicyContext(BaseModel):
    """Execution policy constraints attached to the request."""
    priority: ExecutionPriority = Field(default=ExecutionPriority.NORMAL, description="Request priority")
    timeout_class: TimeoutClass = Field(default=TimeoutClass.STANDARD, description="Timeout class")
    allow_cache: bool = Field(default=True, description="Whether cache lookup is allowed")
    allow_dedup: bool = Field(default=True, description="Whether in-flight deduplication is allowed")
    requested_capability: Optional[str] = Field(default=None, description="Requested AI capability override")
    max_retries: int = Field(default=2, description="Maximum allowed retries on retryable failure")


class AIRequestContext(BaseModel):
    """
    Standard request envelope passed from Go Gateway to the AI Microservice.
    Contains trusted identity, tenant, course, and policy context.
    """
    request_id: str = Field(..., description="Unique request ID for tracing")
    trace_id: str = Field(..., description="Distributed trace ID")
    session_id: Optional[str] = Field(default=None, description="Client conversation/session ID")
    
    principal: PrincipalContext = Field(..., description="Authenticated user principal")
    tenant: TenantContext = Field(default_factory=TenantContext, description="Institution tenant context")
    course: CourseContext = Field(default_factory=CourseContext, description="Course context")
    client: ClientContext = Field(default_factory=ClientContext, description="Client platform info")
    policy: PolicyContext = Field(default_factory=PolicyContext, description="Policy and timeout class")
    
    operation: str = Field(..., description="AI Operation name (e.g. chat.generate, study.flashcards.generate)")
    operation_version: str = Field(default="1", description="Version of the operation schema")
    
    input: Dict[str, Any] = Field(default_factory=dict, description="Operation input data")
    parameters: Dict[str, Any] = Field(default_factory=dict, description="Operation parameters")
    metadata: Dict[str, Any] = Field(default_factory=dict, description="Optional extra metadata")
    
    idempotency_key: Optional[str] = Field(default=None, description="Optional idempotency key")

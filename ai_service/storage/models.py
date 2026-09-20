"""
SQLAlchemy ORM models for LexiAssist AI Microservice.
Multi-tenant schema (ai.*) covering sessions, document chunks (pgvector), jobs, and audit logs.
"""
import enum
from datetime import datetime, timezone
from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    Text,
    DateTime,
    JSON,
    Enum as SAEnum,
    Index,
)
from sqlalchemy.orm import declarative_base

Base = declarative_base()


class SessionType(str, enum.Enum):
    NOTES = "notes"           # Writing assistant
    READING = "reading"       # Reading assistant
    FLASHCARD = "flashcard"   # Study tools
    QUIZ = "quiz"             # Study tools
    CHAT = "chat"             # Chat conversations


class UserSession(Base):
    """
    Multi-tenant session record for user AI interactions.
    Scoped by user_id and optionally by institution_id / course_id.
    """
    __tablename__ = "user_sessions"
    __table_args__ = (
        Index("idx_ai_sessions_user", "user_id"),
        Index("idx_ai_sessions_tenant", "institution_id", "course_id"),
        Index("idx_ai_sessions_type", "session_type"),
        {"schema": "ai"},
    )

    session_id = Column(String, primary_key=True)
    user_id = Column(String, nullable=False, index=True)
    institution_id = Column(String, nullable=True, index=True)
    course_id = Column(String, nullable=True, index=True)
    session_type = Column(SAEnum(SessionType), nullable=False)
    filename = Column(String, nullable=True)
    created_at = Column(DateTime, default=lambda: datetime.now(timezone.utc))
    updated_at = Column(DateTime, default=lambda: datetime.now(timezone.utc), onupdate=lambda: datetime.now(timezone.utc))

    # Writing assistant fields
    subject = Column(String, nullable=True)
    structured_notes = Column(Text, nullable=True)

    # Reading assistant fields
    summary = Column(Text, nullable=True)
    summary_type = Column(String, nullable=True)
    tts_audio_b64 = Column(Text, nullable=True)
    vocab_terms = Column(JSON, nullable=True)

    # Study buddy fields
    flashcards = Column(JSON, nullable=True)
    num_cards = Column(Integer, nullable=True)
    quiz_type = Column(String, nullable=True)
    questions = Column(JSON, nullable=True)
    num_questions = Column(Integer, nullable=True)

    # Chat assistant fields
    conversation_history = Column(JSON, nullable=True)


# Conditional import for pgvector
try:
    from pgvector.sqlalchemy import Vector
    vector_type = Vector(1024)
except ImportError:
    vector_type = JSON  # Fallback for mock/test environments without pgvector C extension


class LexiChunk(Base):
    """
    1024-dimensional semantic document chunk vector index (Cohere embed-multilingual-v3.0).
    Enforces multi-tenant institutional and course boundaries.
    """
    __tablename__ = "lexi_chunks"
    __table_args__ = (
        Index("idx_lexi_chunks_tenant_course", "institution_id", "course"),
        Index("idx_lexi_chunks_doc_id", "doc_id"),
        {"schema": "ai"},
    )

    id = Column(String, primary_key=True)
    doc_id = Column(String, nullable=False, index=True)
    institution_id = Column(String, nullable=True, index=True)
    course = Column(String, nullable=False, index=True)
    chunk_index = Column(Integer, nullable=False)
    chunk_text = Column(Text, nullable=False)
    source = Column(String, nullable=False, default="uploaded_note")
    heading = Column(String, nullable=True)
    section = Column(String, nullable=True)
    embedding = Column(vector_type, nullable=False)
    created_at = Column(DateTime, default=lambda: datetime.now(timezone.utc))


class AIJob(Base):
    """
    Persistent state for asynchronous long-running AI operations.
    """
    __tablename__ = "jobs"
    __table_args__ = (
        Index("idx_ai_jobs_status", "status"),
        Index("idx_ai_jobs_user", "user_id"),
        Index("idx_ai_jobs_tenant", "institution_id"),
        {"schema": "ai"},
    )

    job_id = Column(String, primary_key=True)
    request_id = Column(String, nullable=False, index=True)
    trace_id = Column(String, nullable=False)
    user_id = Column(String, nullable=False, index=True)
    institution_id = Column(String, nullable=True, index=True)
    course_id = Column(String, nullable=True)
    
    task_type = Column(String, nullable=False)
    priority = Column(String, nullable=False, default="normal")
    status = Column(String, nullable=False, default="queued")  # queued, claimed, running, completed, failed, cancelled
    
    payload = Column(JSON, nullable=False)
    result = Column(JSON, nullable=True)
    error = Column(JSON, nullable=True)
    
    progress = Column(Integer, default=0)
    progress_stage = Column(String, default="queued")
    
    retry_count = Column(Integer, default=0)
    max_retries = Column(Integer, default=2)
    
    created_at = Column(DateTime, default=lambda: datetime.now(timezone.utc))
    started_at = Column(DateTime, nullable=True)
    completed_at = Column(DateTime, nullable=True)
    updated_at = Column(DateTime, default=lambda: datetime.now(timezone.utc), onupdate=lambda: datetime.now(timezone.utc))


class AIRequestAuditModel(Base):
    """
    Immutable request audit log for complete explainability and institutional accounting.
    """
    __tablename__ = "request_audits"
    __table_args__ = (
        Index("idx_ai_audits_tenant_date", "institution_id", "created_at"),
        Index("idx_ai_audits_user_date", "user_id", "created_at"),
        Index("idx_ai_audits_op", "operation"),
        {"schema": "ai"},
    )

    id = Column(String, primary_key=True)
    request_id = Column(String, nullable=False, index=True)
    trace_id = Column(String, nullable=False)
    user_id = Column(String, nullable=False, index=True)
    institution_id = Column(String, nullable=True, index=True)
    course_id = Column(String, nullable=True)
    
    operation = Column(String, nullable=False)
    operation_version = Column(String, default="1")
    status = Column(String, nullable=False)
    failure_code = Column(String, nullable=True)
    
    started_at = Column(DateTime, nullable=False)
    completed_at = Column(DateTime, nullable=False)
    
    queue_duration_ms = Column(Integer, default=0)
    retrieval_duration_ms = Column(Integer, default=0)
    tool_duration_ms = Column(Integer, default=0)
    provider_duration_ms = Column(Integer, default=0)
    total_duration_ms = Column(Integer, default=0)
    
    cache_hit = Column(Boolean, default=False)
    cache_scope = Column(String, nullable=True)
    deduplicated = Column(Boolean, default=False)
    
    provider = Column(String, nullable=True)
    model = Column(String, nullable=True)
    
    input_tokens = Column(Integer, default=0)
    output_tokens = Column(Integer, default=0)
    total_tokens = Column(Integer, default=0)
    audio_seconds = Column(Float, default=0.0)
    estimated_cost_usd = Column(Float, default=0.0)
    
    retry_count = Column(Integer, default=0)
    created_at = Column(DateTime, default=lambda: datetime.now(timezone.utc))


class QuizAnswerKey(Base):
    """Answer keys for quiz validation and grading."""
    __tablename__ = "quiz_answer_keys"
    __table_args__ = {"schema": "ai"}

    quiz_id = Column(String, primary_key=True)
    institution_id = Column(String, nullable=True, index=True)
    course_id = Column(String, nullable=True, index=True)
    answers = Column(JSON, nullable=False)
    rubric = Column(JSON, nullable=True)
    created_at = Column(DateTime, default=lambda: datetime.now(timezone.utc))

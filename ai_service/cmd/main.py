"""
Main FastAPI entry point for LexiAssist AI Microservice.
Contains the internal AI Gateway control plane, tool registry, and multi-priority job worker.
"""
import os
import sys
from contextlib import asynccontextmanager
from fastapi import FastAPI, HTTPException, Header, Request, Depends, status
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import StreamingResponse, JSONResponse
from pydantic import BaseModel, Field

# Ensure root directory is on python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../..")))

from ai_service.config import settings
from ai_service.observability.logger import setup_logger
from ai_service.storage.database import init_db
from ai_service.gateway.pipeline import AIGatewayPipeline
from ai_service.contracts.context import AIRequestContext, ExecutionPriority
from ai_service.contracts.execution import AIExecutionEnvelope
from ai_service.jobs.queue_manager import QueueManager
from ai_service.jobs.worker import AsyncAIWorker
from ai_service.observability.cost_tracker import CostTracker

# Domain Tools
from ai_service.tools.chat_tool import ChatTool
from ai_service.tools.reading_tool import ReadingTool
from ai_service.tools.study_tool import StudyTool
from ai_service.tools.writing_tool import WritingTool
from ai_service.tools.ingestion_tool import IngestionTool
from ai_service.tools.retrieval_tool import RetrievalTool
from ai_service.tools.evaluation_tool import EvaluationTool

logger = setup_logger(settings.log_level)

# Initialize AI Gateway control plane
pipeline = AIGatewayPipeline()
pipeline.register_tool("chat", ChatTool())
pipeline.register_tool("reading", ReadingTool())
pipeline.register_tool("study", StudyTool())
pipeline.register_tool("writing", WritingTool())
pipeline.register_tool("document", IngestionTool())
pipeline.register_tool("retrieval", RetrievalTool())
pipeline.register_tool("evaluation", EvaluationTool())

# Initialize Job Manager and Worker
queue_manager = QueueManager()
worker = AsyncAIWorker(pipeline)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Lifespan event: initialize database and start worker."""
    logger.info("Initializing LexiAssist AI Microservice...")
    init_db()
    worker.start()
    yield
    logger.info("Shutting down LexiAssist AI Microservice...")
    worker.stop()


app = FastAPI(
    title="LexiAssist AI Microservice",
    description="Unified AI Control Plane and Domain Capability Engine",
    version="4.0.0",
    lifespan=lifespan,
)

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


async def verify_internal_key(request: Request, x_internal_key: str = Header(None)):
    """Authenticate requests from Go Gateway. Skipped in development/local environments."""
    if request.url.path in ("/", "/health", "/docs", "/openapi.json"):
        return
    # In development mode or local testing, allow requests without requiring internal key header
    if settings.environment == "development" or settings.internal_api_key in ("", "dev-internal-key"):
        return
    if not x_internal_key or x_internal_key != settings.internal_api_key:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or missing X-Internal-Key"
        )


@app.get("/")
async def root():
    return {
        "service": "LexiAssist AI Microservice",
        "version": "4.0.0",
        "control_plane": "Internal AI Gateway",
        "status": "healthy"
    }


@app.get("/health")
async def health():
    """Deep dependency health check."""
    gemini_health = await pipeline.gemini_adapter.health()
    cohere_health = await pipeline.cohere_adapter.health()
    return {
        "status": "healthy",
        "gemini": gemini_health,
        "cohere": cohere_health,
        "circuit_breaker": pipeline.circuit_breaker.state.value,
    }


# ==============================================================================
# Unified AI Execution Contract (Spec § 13)
# ==============================================================================

@app.post(
    "/internal/v1/ai/execute",
    dependencies=[Depends(verify_internal_key)],
    response_model=AIExecutionEnvelope,
)
async def execute_request(ctx: AIRequestContext):
    """
    Unified entry point for synchronous AI operations.
    Executes through the canonical 13-stage AI Gateway pipeline.
    """
    envelope = await pipeline.execute_sync(ctx)
    return envelope


@app.post(
    "/internal/v1/ai/execute/stream",
    dependencies=[Depends(verify_internal_key)],
)
async def execute_stream_request(ctx: AIRequestContext):
    """
    Unified entry point for streaming AI operations.
    Returns standard Server-Sent Events (SSE) stream.
    """
    return StreamingResponse(
        pipeline.execute_stream(ctx),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Request-ID": ctx.request_id,
            "X-Trace-ID": ctx.trace_id,
        },
    )


# ==============================================================================
# Asynchronous Job Execution (Spec §§ 45, 46)
# ==============================================================================

class CreateJobRequest(BaseModel):
    task_type: str = Field(..., description="e.g. reading.analyse, writing.notes, study.flashcards")
    payload: dict = Field(..., description="Job input payload")
    priority: ExecutionPriority = Field(default=ExecutionPriority.NORMAL)
    is_long_running: bool = Field(default=False)
    institution_id: str = Field(default=None)
    course_id: str = Field(default=None)
    user_id: str = Field(..., description="Requesting user ID")


@app.post(
    "/internal/v1/ai/jobs",
    dependencies=[Depends(verify_internal_key)],
)
async def create_job(req: CreateJobRequest):
    """Enqueue a long-running asynchronous AI job."""
    job_id = queue_manager.enqueue_job(
        task_type=req.task_type,
        payload=req.payload,
        user_id=req.user_id,
        institution_id=req.institution_id,
        course_id=req.course_id,
        priority=req.priority,
        is_long_running=req.is_long_running,
    )
    return {"job_id": job_id, "status": "queued"}


@app.get(
    "/internal/v1/ai/jobs/{job_id}",
    dependencies=[Depends(verify_internal_key)],
)
async def get_job_status(job_id: str):
    """Query progress and result of an asynchronous job."""
    job = queue_manager.get_job_status(job_id)
    if not job:
        raise HTTPException(status_code=404, detail="Job not found")
    return job


@app.post(
    "/internal/v1/ai/jobs/{job_id}/cancel",
    dependencies=[Depends(verify_internal_key)],
)
async def cancel_job(job_id: str):
    """Cancel an active or queued job."""
    success = queue_manager.cancel_job(job_id)
    return {"job_id": job_id, "cancelled": success}


# ==============================================================================
# Cost Attribution & Analytics (Spec §§ 55, 56)
# ==============================================================================

@app.get(
    "/internal/v1/ai/analytics/costs",
    dependencies=[Depends(verify_internal_key)],
)
async def get_costs(institution_id: str = None):
    """Query institutional AI expenditure and avoided cache savings."""
    return CostTracker.get_summary_by_institution(institution_id)


if __name__ == "__main__":
    import uvicorn
    uvicorn.run("ai_service.cmd.main:app", host="0.0.0.0", port=settings.port, reload=True)

"""
Multi-Priority Queue Manager for LexiAssist AI Infrastructure.
Provides queue isolation across Interactive, Standard, Long-running, and Background workloads.
"""
import json
import uuid
import logging
from typing import Optional, Dict, Any
from datetime import datetime, timezone

from ai_service.storage.redis_client import get_redis_client
from ai_service.contracts.context import ExecutionPriority
from ai_service.jobs.state_machine import JobStatus
from ai_service.storage.database import get_db_session
from ai_service.storage.models import AIJob

logger = logging.getLogger(__name__)

# Redis Queue Keys by Priority
QUEUE_KEYS = {
    ExecutionPriority.CRITICAL: "ai:queue:interactive",
    ExecutionPriority.INTERACTIVE: "ai:queue:interactive",
    ExecutionPriority.NORMAL: "ai:queue:standard",
    ExecutionPriority.BACKGROUND: "ai:queue:background",
}
LONG_RUNNING_QUEUE = "ai:queue:long_running"
DEAD_LETTER_QUEUE = "ai:queue:dead_letter"


class QueueManager:
    """Manages enqueueing and priority dispatching of asynchronous AI jobs."""

    def __init__(self):
        self.redis = get_redis_client()

    def enqueue_job(
        self,
        task_type: str,
        payload: Dict[str, Any],
        user_id: str,
        institution_id: Optional[str] = None,
        course_id: Optional[str] = None,
        priority: ExecutionPriority = ExecutionPriority.NORMAL,
        is_long_running: bool = False,
    ) -> str:
        """Create a new job in DB and push its ID to the appropriate Redis queue."""
        job_id = str(uuid.uuid4())
        request_id = payload.get("request_id", str(uuid.uuid4()))
        trace_id = payload.get("trace_id", str(uuid.uuid4()))

        # Determine target queue
        if is_long_running:
            target_queue = LONG_RUNNING_QUEUE
        else:
            target_queue = QUEUE_KEYS.get(priority, "ai:queue:standard")

        # 1. Persist to PostgreSQL
        with get_db_session() as session:
            db_job = AIJob(
                job_id=job_id,
                request_id=request_id,
                trace_id=trace_id,
                user_id=user_id,
                institution_id=institution_id,
                course_id=course_id,
                task_type=task_type,
                priority=priority.value,
                status=JobStatus.QUEUED.value,
                payload=payload,
                progress=0,
                progress_stage="queued",
            )
            session.add(db_job)

        # 2. Push to Redis queue
        if self.redis:
            try:
                # Also store quick lookup hash in Redis
                self.redis.hset(
                    f"ai:job:{job_id}",
                    mapping={
                        "status": JobStatus.QUEUED.value,
                        "task_type": task_type,
                        "user_id": user_id,
                        "progress": "0",
                        "updated_at": datetime.now(timezone.utc).isoformat(),
                    }
                )
                self.redis.lpush(target_queue, job_id)
                logger.info(f"📥 Enqueued job {job_id} into {target_queue}")
            except Exception as e:
                logger.error(f"Redis enqueue failed: {e}")

        return job_id

    def get_job_status(self, job_id: str) -> Optional[Dict[str, Any]]:
        """Get live job status and progress from Redis/DB."""
        if self.redis:
            try:
                cached = self.redis.hgetall(f"ai:job:{job_id}")
                if cached:
                    return cached
            except Exception:
                pass

        with get_db_session() as session:
            db_job = session.query(AIJob).filter(AIJob.job_id == job_id).first()
            if not db_job:
                return None
            return {
                "job_id": db_job.job_id,
                "status": db_job.status,
                "progress": db_job.progress,
                "progress_stage": db_job.progress_stage,
                "result": db_job.result,
                "error": db_job.error,
                "created_at": db_job.created_at.isoformat() if db_job.created_at else None,
            }

    def cancel_job(self, job_id: str) -> bool:
        """Mark a job as cancelled."""
        if self.redis:
            self.redis.hset(f"ai:job:{job_id}", "status", JobStatus.CANCELLED.value)

        with get_db_session() as session:
            db_job = session.query(AIJob).filter(AIJob.job_id == job_id).first()
            if db_job:
                db_job.status = JobStatus.CANCELLED.value
                return True
        return False

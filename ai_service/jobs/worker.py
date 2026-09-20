"""
Async Background Job Worker for LexiAssist AI Infrastructure.
Pulls from priority queues, manages execution lifecycle, updates genuine progress, and handles cancellation.
"""
import json
import asyncio
import threading
import logging
from typing import Optional, Dict, Any
from datetime import datetime, timezone

from ai_service.storage.redis_client import get_redis_client
from ai_service.jobs.state_machine import JobStatus
from ai_service.storage.database import get_db_session
from ai_service.storage.models import AIJob

logger = logging.getLogger(__name__)

# Priority order for popping jobs
ACTIVE_QUEUES = [
    "ai:queue:interactive",
    "ai:queue:standard",
    "ai:queue:long_running",
    "ai:queue:background",
]


class AsyncAIWorker:
    """Daemon thread worker consuming Redis priority queues."""

    def __init__(self, pipeline: Any):
        self.pipeline = pipeline
        self.redis = get_redis_client()
        self._stop_event = threading.Event()
        self._thread: Optional[threading.Thread] = None

    def start(self):
        """Start worker background thread."""
        if self._thread is not None and self._thread.is_alive():
            return
        self._stop_event.clear()
        self._thread = threading.Thread(target=self._run_loop, daemon=True)
        self._thread.start()
        logger.info("🚀 AI Background Worker started")

    def stop(self):
        """Signal worker to stop gracefully."""
        self._stop_event.set()
        if self._thread is not None:
            self._thread.join(timeout=10)
        logger.info("🛑 AI Background Worker stopped")

    def _run_loop(self):
        """Polling loop for Redis queue items."""
        while not self._stop_event.is_set():
            if not self.redis:
                self._stop_event.wait(5)
                self.redis = get_redis_client()
                continue

            try:
                # Blocking pop across priority queues (BRPOP returns (queue_name, item))
                item = self.redis.brpop(ACTIVE_QUEUES, timeout=2)
                if not item:
                    continue

                queue_name, job_id = item
                self._process_job(job_id)

            except Exception as e:
                logger.error(f"Worker loop error: {e}")
                self._stop_event.wait(2)

    def _update_job(self, job_id: str, status: JobStatus, progress: int, stage: str, result=None, error=None):
        now_str = datetime.now(timezone.utc).isoformat()
        if self.redis:
            try:
                mapping = {
                    "status": status.value,
                    "progress": str(progress),
                    "progress_stage": stage,
                    "updated_at": now_str,
                }
                if result:
                    mapping["result"] = json.dumps(result)
                if error:
                    mapping["error"] = str(error)
                self.redis.hset(f"ai:job:{job_id}", mapping=mapping)
            except Exception:
                pass

        with get_db_session() as session:
            db_job = session.query(AIJob).filter(AIJob.job_id == job_id).first()
            if db_job:
                db_job.status = status.value
                db_job.progress = progress
                db_job.progress_stage = stage
                if result:
                    db_job.result = result
                if error:
                    db_job.error = {"message": str(error)}
                if status == JobStatus.RUNNING and not db_job.started_at:
                    db_job.started_at = datetime.now(timezone.utc)
                if status in (JobStatus.COMPLETED, JobStatus.FAILED, JobStatus.CANCELLED):
                    db_job.completed_at = datetime.now(timezone.utc)

    def _process_job(self, job_id: str):
        """Execute a claimed job."""
        # 1. Check if cancelled before claiming
        with get_db_session() as session:
            db_job = session.query(AIJob).filter(AIJob.job_id == job_id).first()
            if not db_job or db_job.status == JobStatus.CANCELLED.value:
                return
            task_type = db_job.task_type
            payload = db_job.payload or {}
            user_id = db_job.user_id

        self._update_job(job_id, JobStatus.RUNNING, 10, "Claimed by worker")
        logger.info(f"⚙️ Worker processing job [{job_id}] type: {task_type}")

        try:
            # Reconstruct AIRequestContext from payload
            from ai_service.contracts.context import (
                AIRequestContext,
                PrincipalContext,
                TenantContext,
                CourseContext,
                PolicyContext,
            )

            ctx = AIRequestContext(
                request_id=payload.get("request_id", job_id),
                trace_id=payload.get("trace_id", job_id),
                principal=PrincipalContext(user_id=user_id),
                tenant=TenantContext(institution_id=payload.get("institution_id")),
                course=CourseContext(course_id=payload.get("course_id")),
                operation=task_type,
                input=payload.get("input", payload),
                parameters=payload.get("parameters", {}),
            )

            self._update_job(job_id, JobStatus.RUNNING, 40, "Executing AI pipeline")

            # Run in asyncio event loop
            loop = asyncio.new_event_loop()
            asyncio.set_event_loop(loop)
            try:
                envelope = loop.run_until_complete(self.pipeline.execute_sync(ctx))
            finally:
                loop.close()

            if envelope.is_success():
                self._update_job(job_id, JobStatus.COMPLETED, 100, "Completed successfully", result=envelope.result)
                logger.info(f"✅ Completed job [{job_id}]")
            else:
                err_msg = envelope.error.message if envelope.error else "Execution failed"
                self._update_job(job_id, JobStatus.FAILED, 0, "Execution failed", error=err_msg)
                logger.warning(f"❌ Job [{job_id}] failed: {err_msg}")

        except Exception as e:
            logger.error(f"❌ Exception executing job [{job_id}]: {e}", exc_info=True)
            self._update_job(job_id, JobStatus.FAILED, 0, "Internal error", error=str(e))

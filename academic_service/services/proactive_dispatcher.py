"""
Proactive Notification Dispatcher for Lexi Academic Service.
Connects ProactiveEngine decisions directly to the Go Notification Service (:8084)
for push (FCM) and email (SMTP) dispatches.
"""
import os
import logging
from typing import Dict, Any, Optional
import httpx

from academic_service.models.orm import ProactiveIntervention

logger = logging.getLogger(__name__)

NOTIFICATION_SERVICE_URL = os.getenv("NOTIFICATION_SERVICE_URL", "http://localhost:8084")
INTERNAL_API_KEY = os.getenv("INTERNAL_API_KEY", "dev-internal-key")


class ProactiveDispatcher:
    """Dispatches critical timeline cards and proactive events to student devices."""

    @classmethod
    async def dispatch_event(
        cls,
        user_id: str,
        event_type: str,
        title: str,
        body: str,
        data: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """Send an event payload to Go notification-service internal event queue."""
        payload = {
            "user_id": user_id,
            "event_type": event_type,
            "title": title,
            "body": body,
            "data": data or {},
        }
        headers = {
            "X-Internal-Key": INTERNAL_API_KEY,
            "Content-Type": "application/json",
        }

        url = f"{NOTIFICATION_SERVICE_URL.rstrip('/')}/api/v1/internal/events"
        try:
            async with httpx.AsyncClient(timeout=10.0) as client:
                resp = await client.post(url, json=payload, headers=headers)
                if resp.status_code in (200, 201, 202):
                    logger.info(f"Proactive event dispatched for user {user_id}: {title}")
                    return {"status": "delivered", "status_code": resp.status_code}
                else:
                    logger.warning(f"Notification service responded with {resp.status_code}: {resp.text}")
                    return {"status": "error", "status_code": resp.status_code, "detail": resp.text}
        except Exception as e:
            logger.warning(f"Notification service dispatch failed (service might be offline in dev): {e}")
            return {"status": "fallback_local", "error": str(e)}

    @classmethod
    async def dispatch_intervention_alert(
        cls,
        intervention: ProactiveIntervention,
    ) -> Dict[str, Any]:
        """Dispatch a high-priority intervention decision to notification service."""
        payload = intervention.payload or {}
        card_type = intervention.card_type
        
        # Build student-friendly body
        body = payload.get("action_label", "Open Lexi to see your next study step.")
        if card_type == "pre_class_prep":
            time_until = payload.get("time_until", "shortly")
            concepts = payload.get("key_concepts", [])
            body = f"Class starts in {time_until}. Review {len(concepts)} key concepts now."
        elif card_type == "gap_diagnostic":
            topic = payload.get("topic", "your course")
            body = f"You have a knowledge gap in {topic}. Spend 3 mins to master it."
        elif card_type == "deadline_alert":
            title = payload.get("title", "Upcoming assessment")
            body = f"Due soon: {title}. Tap to start preparation."

        data = {
            "intervention_id": intervention.id,
            "card_type": intervention.card_type,
            "course_id": intervention.course_id or "",
            "action_type": payload.get("action_type", "open_today"),
        }

        return await cls.dispatch_event(
            user_id=intervention.user_id,
            event_type=intervention.event_type,
            title=intervention.title,
            body=body,
            data=data,
        )


"""Unit tests for Proactive Notification Dispatcher."""
import pytest
from unittest.mock import AsyncMock, patch
from academic_service.services.proactive_dispatcher import ProactiveDispatcher
from academic_service.models.orm import ProactiveIntervention


@pytest.mark.anyio
async def test_dispatch_intervention_alert():
    intervention = ProactiveIntervention(
        id="int_test_123",
        user_id="user_test_456",
        course_id="course_csc301_veritas",
        event_type="pre_class_prep",
        card_type="pre_class_prep",
        title="📚 CSC 301 in 45 mins",
        payload={
            "time_until": "45 mins",
            "key_concepts": ["Base Cases", "Memoization"],
            "action_type": "open_partner_deck",
        },
    )

    with patch("httpx.AsyncClient.post", new_callable=AsyncMock) as mock_post:
        mock_post.return_value.status_code = 202
        res = await ProactiveDispatcher.dispatch_intervention_alert(intervention)
        assert res["status"] == "delivered"
        assert res["status_code"] == 202


"""
Unit tests for AI Gateway Pipeline and Caching / Dedup Managers.
"""
import pytest
import asyncio
from unittest.mock import AsyncMock, MagicMock
from ai_service.contracts.context import (
    AIRequestContext,
    PrincipalContext,
    TenantContext,
    CourseContext,
)
from ai_service.contracts.execution import ExecutionState
from ai_service.gateway.pipeline import AIGatewayPipeline
from ai_service.gateway.cache_manager import CacheManager, CacheScope
from ai_service.gateway.dedup_manager import DedupManager
from ai_service.gateway.circuit_breaker import CircuitBreaker, CircuitState


@pytest.mark.anyio
async def test_cache_manager_key_computation():
    mgr = CacheManager()
    key1 = mgr.compute_cache_key(
        scope=CacheScope.COURSE,
        scope_id="uni_lagos:CSC301",
        operation="study.flashcards.generate",
        operation_version="1",
        payload_data={"text": "Data structures"},
    )
    key2 = mgr.compute_cache_key(
        scope=CacheScope.COURSE,
        scope_id="uni_lagos:CSC301",
        operation="study.flashcards.generate",
        operation_version="1",
        payload_data={"text": "Data structures"},
    )
    key3 = mgr.compute_cache_key(
        scope=CacheScope.COURSE,
        scope_id="uni_covenant:CSC301",
        operation="study.flashcards.generate",
        operation_version="1",
        payload_data={"text": "Data structures"},
    )

    assert key1 == key2  # Deterministic
    assert key1 != key3  # Tenant isolation verified


def test_circuit_breaker_transitions():
    cb = CircuitBreaker(failure_threshold=3, recovery_timeout_seconds=0.1)
    assert cb.state == CircuitState.CLOSED
    assert cb.allow_request() is True

    # 3 failures -> trip
    cb.record_failure()
    cb.record_failure()
    cb.record_failure()
    assert cb.state == CircuitState.OPEN
    assert cb.allow_request() is False


@pytest.mark.anyio
async def test_dedup_manager_local_collapsing():
    dedup = DedupManager()
    key = "ai_dedup:test_op:123"

    is_primary, shared = await dedup.acquire_or_wait(key)
    assert is_primary is True
    assert shared is None

    # Simulate completed execution broadcast
    await dedup.complete_and_broadcast(key, {"status": "done"})

"""
Canonical 13-Stage AI Gateway Execution Pipeline for LexiAssist AI Infrastructure.
Coordinates request classification, quotas, caching, deduplication, routing, and telemetry.
"""
import time
import logging
from typing import Optional, Dict, Any, AsyncGenerator
from datetime import datetime, timezone

from ai_service.contracts.context import AIRequestContext, ExecutionPriority, TimeoutClass
from ai_service.contracts.execution import (
    AIExecutionEnvelope,
    ExecutionMode,
    ExecutionState,
    ExecutionMetadata,
    SourceInfo,
)
from ai_service.contracts.errors import (
    FailureCode,
    FailureStage,
    AIErrorDetail,
)
from ai_service.contracts.usage import TokenUsage, TimingMetrics, AIUsageRecord
from ai_service.contracts.events import (
    StreamEventType,
    StreamStartedEvent,
    StreamThinkingEvent,
    StreamTokenEvent,
    StreamCompletedEvent,
    StreamErrorEvent,
)

from ai_service.gateway.quota_manager import QuotaManager
from ai_service.gateway.rate_limiter import RateLimiter
from ai_service.gateway.cache_manager import CacheManager, CacheScope
from ai_service.gateway.dedup_manager import DedupManager
from ai_service.gateway.timeout_manager import TimeoutManager
from ai_service.gateway.retry_manager import RetryManager
from ai_service.gateway.circuit_breaker import CircuitBreaker
from ai_service.gateway.model_router import ModelRouter
from ai_service.gateway.cancellation import CancellationManager

from ai_service.adapters.gemini_adapter import GeminiAdapter
from ai_service.adapters.cohere_adapter import CohereEmbeddingAdapter
from ai_service.adapters.audio_adapter import AudioAdapter

logger = logging.getLogger(__name__)


class AIGatewayPipeline:
    """The central control plane for all AI computations in LexiAssist."""

    def __init__(self):
        self.quota_mgr = QuotaManager()
        self.rate_limiter = RateLimiter()
        self.cache_mgr = CacheManager()
        self.dedup_mgr = DedupManager()
        self.timeout_mgr = TimeoutManager()
        self.retry_mgr = RetryManager()
        self.circuit_breaker = CircuitBreaker()
        self.model_router = ModelRouter()
        self.cancellation_mgr = CancellationManager()
        
        # Provider Adapters
        self.gemini_adapter = GeminiAdapter()
        self.cohere_adapter = CohereEmbeddingAdapter()
        self.audio_adapter = AudioAdapter()

        # Tools registry (injected on startup)
        self._tools: Dict[str, Any] = {}

    def register_tool(self, operation_prefix: str, tool_instance: Any):
        """Register a domain capability tool."""
        self._tools[operation_prefix] = tool_instance
        logger.info(f"Registered AI tool: {operation_prefix}")

    def _determine_cache_scope(self, ctx: AIRequestContext) -> Tuple[CacheScope, str]:
        """Determine scope for caching based on tenant and course presence."""
        if ctx.tenant.institution_id and ctx.course.course_id:
            return CacheScope.COURSE, f"{ctx.tenant.institution_id}:{ctx.course.course_id}"
        if ctx.tenant.institution_id:
            return CacheScope.INSTITUTION, ctx.tenant.institution_id
        return CacheScope.USER, ctx.principal.user_id

    async def execute_sync(self, ctx: AIRequestContext) -> AIExecutionEnvelope:
        """
        Execute synchronous AI request through the canonical pipeline.
        """
        start_time = time.time()
        timings = TimingMetrics()
        
        # 1. Validation
        if not ctx.request_id or not ctx.operation:
            return self._build_error_envelope(
                ctx, FailureCode.VALIDATION_ERROR, FailureStage.VALIDATION,
                "Invalid request: missing request_id or operation", False, timings, start_time
            )

        # 2. Quota Check
        allowed, reason = await self.quota_mgr.check_and_reserve_quota(
            ctx.principal.user_id, ctx.tenant.institution_id
        )
        if not allowed:
            return self._build_error_envelope(
                ctx, FailureCode.QUOTA_EXCEEDED, FailureStage.QUOTA_CHECK,
                reason or "Daily AI quota exceeded", False, timings, start_time
            )

        # 3. Rate Limit Check
        is_limited, _ = await self.rate_limiter.is_rate_limited(ctx.principal.user_id, ctx.operation)
        if is_limited:
            return self._build_error_envelope(
                ctx, FailureCode.RATE_LIMITED, FailureStage.RATE_LIMIT_CHECK,
                "Too many requests. Please slow down.", True, timings, start_time
            )

        # 4. Scoped Cache Check
        scope, scope_id = self._determine_cache_scope(ctx)
        payload_data = {"input": ctx.input, "params": ctx.parameters}
        
        if ctx.policy.allow_cache:
            cache_start = time.time()
            cached_result = await self.cache_mgr.get_cached_result(
                scope, scope_id, ctx.operation, ctx.operation_version, payload_data
            )
            if cached_result:
                total_duration = int((time.time() - start_time) * 1000)
                timings.total_duration_ms = total_duration
                return AIExecutionEnvelope(
                    request_id=ctx.request_id,
                    trace_id=ctx.trace_id,
                    status=ExecutionState.COMPLETED,
                    result=cached_result,
                    execution=ExecutionMetadata(
                        mode=ExecutionMode.SYNC,
                        state=ExecutionState.COMPLETED,
                        completed_at=datetime.now(timezone.utc).isoformat(),
                        duration_ms=total_duration,
                        timings=timings,
                    ),
                    usage=TokenUsage(total_tokens=0, estimated_cost_usd=0.0),
                    source=SourceInfo(type="cache", cached=True, cache_scope=scope.value),
                )

        # 5. In-Flight Deduplication Check
        dedup_key = self.dedup_mgr.compute_dedup_key(ctx.operation, payload_data)
        is_primary = True
        if ctx.policy.allow_dedup:
            is_primary, shared_result = await self.dedup_mgr.acquire_or_wait(dedup_key)
            if not is_primary and shared_result:
                total_duration = int((time.time() - start_time) * 1000)
                timings.total_duration_ms = total_duration
                return AIExecutionEnvelope(
                    request_id=ctx.request_id,
                    trace_id=ctx.trace_id,
                    status=ExecutionState.COMPLETED,
                    result=shared_result,
                    execution=ExecutionMetadata(
                        mode=ExecutionMode.SYNC,
                        state=ExecutionState.COMPLETED,
                        completed_at=datetime.now(timezone.utc).isoformat(),
                        duration_ms=total_duration,
                        timings=timings,
                    ),
                    usage=TokenUsage(total_tokens=0, estimated_cost_usd=0.0),
                    source=SourceInfo(type="deduplication", deduplicated=True),
                )

        # 6. Circuit Breaker Check
        if not self.circuit_breaker.allow_request():
            return self._build_error_envelope(
                ctx, FailureCode.PROVIDER_UNAVAILABLE, FailureStage.MODEL_EXECUTION,
                "AI Provider is temporarily unavailable due to high error rates (Circuit Breaker OPEN)", True, timings, start_time
            )

        # 7. Execute Domain Tool
        try:
            tool = self._resolve_tool(ctx.operation)
            if not tool:
                return self._build_error_envelope(
                    ctx, FailureCode.UNSUPPORTED_OPERATION, FailureStage.TOOL_EXECUTION,
                    f"Unsupported AI operation: {ctx.operation}", False, timings, start_time
                )

            tool_start = time.time()
            result, usage, model_info = await tool.execute(ctx, self)
            timings.tool_ms = int((time.time() - tool_start) * 1000)
            
            # Record success in circuit breaker
            self.circuit_breaker.record_success()

            # 8. Store in Scoped Cache
            if ctx.policy.allow_cache and result:
                await self.cache_mgr.set_cached_result(
                    scope, scope_id, ctx.operation, ctx.operation_version, payload_data, result
                )

            # 9. Broadcast to Deduplicated Waiters
            if ctx.policy.allow_dedup:
                await self.dedup_mgr.complete_and_broadcast(dedup_key, result)

            total_duration = int((time.time() - start_time) * 1000)
            timings.total_duration_ms = total_duration

            return AIExecutionEnvelope(
                request_id=ctx.request_id,
                trace_id=ctx.trace_id,
                status=ExecutionState.COMPLETED,
                result=result,
                execution=ExecutionMetadata(
                    mode=ExecutionMode.SYNC,
                    state=ExecutionState.COMPLETED,
                    completed_at=datetime.now(timezone.utc).isoformat(),
                    duration_ms=total_duration,
                    timings=timings,
                ),
                usage=usage,
                source=SourceInfo(
                    type="model",
                    cached=False,
                    provider=model_info.get("provider", "google"),
                    model=model_info.get("model", "gemini-2.5-flash"),
                ),
            )

        except Exception as e:
            self.circuit_breaker.record_failure()
            if ctx.policy.allow_dedup:
                await self.dedup_mgr.complete_and_broadcast(dedup_key, None)

            logger.error(f"❌ AI Execution failed for {ctx.operation}: {e}", exc_info=True)
            return self._build_error_envelope(
                ctx, FailureCode.INTERNAL_ERROR, FailureStage.MODEL_EXECUTION,
                f"AI computation failed: {str(e)}", True, timings, start_time
            )

    async def execute_stream(self, ctx: AIRequestContext) -> AsyncGenerator[str, None]:
        """
        Execute streaming AI request yielding SSE events.
        """
        start_time = time.time()
        
        # Emit initial started event
        yield f"event: started\ndata: {StreamStartedEvent(request_id=ctx.request_id, trace_id=ctx.trace_id, session_id=ctx.session_id, operation=ctx.operation).model_dump_json()}\n\n"

        tool = self._resolve_tool(ctx.operation)
        if not tool or not hasattr(tool, "execute_stream"):
            yield f"event: error\ndata: {StreamErrorEvent(request_id=ctx.request_id, trace_id=ctx.trace_id, failure_code=FailureCode.UNSUPPORTED_OPERATION, message=f'Streaming not supported for {ctx.operation}').model_dump_json()}\n\n"
            return

        try:
            token_idx = 0
            final_usage = TokenUsage()

            async for chunk in tool.execute_stream(ctx, self):
                if chunk.is_final:
                    final_usage = TokenUsage(
                        input_tokens=chunk.input_tokens,
                        output_tokens=chunk.output_tokens,
                        total_tokens=chunk.input_tokens + chunk.output_tokens,
                        estimated_cost_usd=chunk.cost_usd,
                    )
                else:
                    token_idx += 1
                    event = StreamTokenEvent(token=chunk.token, index=token_idx)
                    yield f"event: token\ndata: {event.model_dump_json()}\n\n"

            # Emit completed event
            duration = int((time.time() - start_time) * 1000)
            completed_event = StreamCompletedEvent(
                request_id=ctx.request_id,
                trace_id=ctx.trace_id,
                usage=final_usage,
                duration_ms=duration,
            )
            yield f"event: completed\ndata: {completed_event.model_dump_json()}\n\n"

        except Exception as e:
            logger.error(f"❌ Streaming error: {e}")
            error_event = StreamErrorEvent(
                request_id=ctx.request_id,
                trace_id=ctx.trace_id,
                failure_code=FailureCode.INTERNAL_ERROR,
                message=str(e),
                retryable=True,
            )
            yield f"event: error\ndata: {error_event.model_dump_json()}\n\n"

    def _resolve_tool(self, operation: str) -> Optional[Any]:
        """Find registered tool matching operation prefix."""
        for prefix, tool in self._tools.items():
            if operation.startswith(prefix):
                return tool
        return None

    def _build_error_envelope(
        self,
        ctx: AIRequestContext,
        code: FailureCode,
        stage: FailureStage,
        message: str,
        retryable: bool,
        timings: TimingMetrics,
        start_time: float,
    ) -> AIExecutionEnvelope:
        duration = int((time.time() - start_time) * 1000)
        timings.total_duration_ms = duration
        
        return AIExecutionEnvelope(
            request_id=ctx.request_id,
            trace_id=ctx.trace_id,
            status=ExecutionState.REJECTED if stage in (FailureStage.VALIDATION, FailureStage.QUOTA_CHECK, FailureStage.AUTHORIZATION) else ExecutionState.FAILED,
            error=AIErrorDetail(
                code=code,
                stage=stage,
                message=message,
                retryable=retryable,
            ),
            execution=ExecutionMetadata(
                mode=ExecutionMode.SYNC,
                state=ExecutionState.FAILED,
                completed_at=datetime.now(timezone.utc).isoformat(),
                duration_ms=duration,
                timings=timings,
            ),
        )

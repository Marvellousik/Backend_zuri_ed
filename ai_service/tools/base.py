"""
Base Tool interface for LexiAssist AI Infrastructure.
Ensures tools are dumb about infrastructure (no direct provider keys, no direct redis logic).
"""
from abc import ABC, abstractmethod
from typing import Dict, Any, Tuple, AsyncGenerator
from ai_service.contracts.context import AIRequestContext
from ai_service.contracts.usage import TokenUsage
from ai_service.adapters.base import StreamChunk


class BaseTool(ABC):
    """Abstract base for all domain AI capabilities."""

    @property
    @abstractmethod
    def operation_prefix(self) -> str:
        """Operation prefix this tool handles (e.g., 'chat', 'reading', 'study')."""
        pass

    @abstractmethod
    async def execute(
        self,
        ctx: AIRequestContext,
        gateway: Any,  # Reference to AIGatewayPipeline for model capability requests
    ) -> Tuple[Dict[str, Any], TokenUsage, Dict[str, str]]:
        """
        Execute domain logic and return (result_dict, token_usage, model_info).
        """
        pass

    async def execute_stream(
        self,
        ctx: AIRequestContext,
        gateway: Any,
    ) -> AsyncGenerator[StreamChunk, None]:
        """Optional streaming execution for real-time tools."""
        raise NotImplementedError("Streaming is not supported by this tool")

"""
Provider and model abstraction interfaces for LexiAssist AI Infrastructure.
Decouples domain capabilities from specific AI vendors (Gemini, Cohere, OpenAI, Local).
"""
from abc import ABC, abstractmethod
from enum import Enum
from typing import Optional, Dict, Any, List, AsyncGenerator
from pydantic import BaseModel, Field


class ModelCapability(str, Enum):
    FAST_CHAT = "fast_chat"                     # Low-latency interactive chat
    DEEP_REASONING = "deep_reasoning"           # Complex academic problem solving
    STRUCTURED_GENERATION = "structured_generation" # Clean JSON flashcards/quizzes
    SUMMARIZATION = "summarization"             # Text summarization & key concepts
    EMBEDDING = "embedding"                     # Vector semantic search
    SPEECH_TO_TEXT = "speech_to_text"           # Audio transcription
    TEXT_TO_SPEECH = "text_to_speech"           # Voice synthesis


class ModelRequirements(BaseModel):
    """Specification of requirements for capability-based model selection."""
    capability: ModelCapability = Field(..., description="Required model capability")
    json_output: bool = Field(default=False, description="Whether valid JSON format is strictly required")
    min_context_window: int = Field(default=4000, description="Minimum context window in tokens")
    cost_sensitive: bool = Field(default=True, description="Prefer cheapest model meeting requirements")
    low_latency: bool = Field(default=False, description="Prioritize TTFT and interactive latency")
    preferred_model: Optional[str] = Field(default=None, description="Explicit model override if permitted")


class GenerationResponse(BaseModel):
    """Normalized response from any LLM provider adapter."""
    content: str
    parsed_json: Optional[Dict[str, Any]] = None
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0
    cost_usd: float = 0.0
    model_used: str
    provider_used: str
    finish_reason: Optional[str] = "stop"
    raw_response: Optional[Any] = None


class StreamChunk(BaseModel):
    """Single token or update chunk emitted during streaming generation."""
    token: str
    is_final: bool = False
    input_tokens: int = 0
    output_tokens: int = 0
    cost_usd: float = 0.0
    model_used: str = ""
    finish_reason: Optional[str] = None


class BaseProviderAdapter(ABC):
    """Abstract interface for all model and external AI provider adapters."""

    @abstractmethod
    async def generate(
        self,
        prompt: str,
        system_instruction: Optional[str] = None,
        model_name: Optional[str] = None,
        temperature: float = 0.4,
        response_mime_type: Optional[str] = None,
        max_output_tokens: Optional[int] = None,
        deadline_budget_seconds: Optional[float] = None,
    ) -> GenerationResponse:
        """Execute synchronous model generation."""
        pass

    @abstractmethod
    async def generate_stream(
        self,
        prompt: str,
        system_instruction: Optional[str] = None,
        model_name: Optional[str] = None,
        temperature: float = 0.4,
        max_output_tokens: Optional[int] = None,
    ) -> AsyncGenerator[StreamChunk, None]:
        """Execute streaming model generation yielding StreamChunk events."""
        pass

    @abstractmethod
    async def health(self) -> Dict[str, Any]:
        """Check provider health, connectivity, and available quota."""
        pass

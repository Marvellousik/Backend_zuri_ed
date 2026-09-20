"""
Adapters package for LexiAssist AI Infrastructure.
"""
from ai_service.adapters.base import (
    BaseProviderAdapter,
    ModelCapability,
    ModelRequirements,
    GenerationResponse,
    StreamChunk,
)
from ai_service.adapters.gemini_adapter import GeminiAdapter
from ai_service.adapters.cohere_adapter import CohereEmbeddingAdapter
from ai_service.adapters.audio_adapter import AudioAdapter
from ai_service.adapters.credentials_pool import CredentialsPool

__all__ = [
    "BaseProviderAdapter",
    "ModelCapability",
    "ModelRequirements",
    "GenerationResponse",
    "StreamChunk",
    "GeminiAdapter",
    "CohereEmbeddingAdapter",
    "AudioAdapter",
    "CredentialsPool",
]

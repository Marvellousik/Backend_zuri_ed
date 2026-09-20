"""
Capability-based Model Router for LexiAssist AI Infrastructure.
Resolves tool requirements into concrete models and provider selection.
"""
import os
import logging
from typing import Optional

from ai_service.adapters.base import ModelCapability, ModelRequirements

logger = logging.getLogger(__name__)

DEFAULT_MODEL = os.getenv("DEFAULT_MODEL", "gemini-2.5-flash-lite")


class ModelRouter:
    """Selects suitable models based on abstract capability requirements and context constraints."""

    def resolve_model(
        self,
        requirements: ModelRequirements,
        estimated_input_chars: int = 0,
    ) -> str:
        """
        Map ModelRequirements to a concrete model identifier.
        """
        # If an explicit model override was requested and valid
        if requirements.preferred_model:
            return requirements.preferred_model

        capability = requirements.capability

        if capability == ModelCapability.DEEP_REASONING:
            return "gemini-2.5-pro"

        if capability == ModelCapability.FAST_CHAT:
            # Low latency interactive chat
            if estimated_input_chars > 15000:
                return "gemini-2.5-flash"
            return "gemini-2.5-flash-lite"

        if capability == ModelCapability.STRUCTURED_GENERATION:
            # Flashcards, quizzes require reliable JSON schema adhering models
            if estimated_input_chars > 20000:
                return "gemini-2.5-pro"
            return "gemini-2.5-flash"

        if capability == ModelCapability.SUMMARIZATION:
            if estimated_input_chars > 25000:
                return "gemini-2.5-pro"
            return "gemini-2.5-flash"

        # Default fallback
        return DEFAULT_MODEL

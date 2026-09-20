"""
Credentials and provider capacity manager for LexiAssist AI Infrastructure.
Keeps API credentials isolated inside the infrastructure without ever exposing them to clients.
"""
import os
import logging
from typing import Dict, Any, Optional

logger = logging.getLogger(__name__)


class CredentialsPool:
    """Manages secure provider credentials and track health."""

    def __init__(self):
        self._credentials: Dict[str, str] = {}
        self._load_env_credentials()

    def _load_env_credentials(self):
        gemini_key = os.getenv("GEMINI_API_KEY") or os.getenv("GOOGLE_API_KEY")
        if gemini_key:
            self._credentials["gemini"] = gemini_key

        cohere_key = os.getenv("COHERE_API_KEY")
        if cohere_key:
            self._credentials["cohere"] = cohere_key

        groq_key = os.getenv("GROQ_API_KEY")
        if groq_key:
            self._credentials["groq"] = groq_key

    def get_key(self, provider: str) -> Optional[str]:
        return self._credentials.get(provider.lower())

    def has_provider(self, provider: str) -> bool:
        return provider.lower() in self._credentials

    def status(self) -> Dict[str, bool]:
        """Return provider configuration status without leaking keys."""
        return {provider: True for provider in self._credentials}

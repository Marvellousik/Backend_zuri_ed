"""
Cohere Embedding Provider Adapter for LexiAssist AI Infrastructure.
Generates 1024-dimensional multilingual embeddings for document chunks and search queries.
"""
import os
import logging
from typing import List, Dict, Any, Optional
import httpx

logger = logging.getLogger(__name__)

COHERE_MODEL = os.getenv("COHERE_EMBED_MODEL", "embed-multilingual-v3.0")
EMBEDDING_DIM = 1024


class CohereEmbeddingAdapter:
    """Adapter for generating vector embeddings via Cohere API."""

    def __init__(self, api_key: Optional[str] = None):
        self.api_key = api_key or os.getenv("COHERE_API_KEY")
        self.model = COHERE_MODEL
        if not self.api_key:
            logger.warning("⚠️ COHERE_API_KEY is not configured. Vector embeddings will fall back or fail.")

    async def embed_texts(
        self,
        texts: List[str],
        input_type: str = "search_document"
    ) -> List[List[float]]:
        """
        Generate embeddings for a list of texts.
        input_type: "search_document" for indexed materials, "search_query" for user queries.
        """
        if not texts:
            return []

        if not self.api_key:
            raise RuntimeError("Missing COHERE_API_KEY for embedding generation")

        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }
        payload = {
            "texts": texts,
            "model": self.model,
            "input_type": input_type,
        }

        async with httpx.AsyncClient(timeout=60.0) as client:
            resp = await client.post("https://api.cohere.ai/v1/embed", json=payload, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            return data.get("embeddings", [])

    async def embed_query(self, query: str) -> List[float]:
        """Convenience method for a single user query."""
        results = await self.embed_texts([query], input_type="search_query")
        if not results:
            raise RuntimeError("Failed to generate query embedding from Cohere")
        return results[0]

    async def health(self) -> Dict[str, Any]:
        return {
            "provider": "cohere",
            "configured": bool(self.api_key),
            "model": self.model,
            "dimension": EMBEDDING_DIM,
        }

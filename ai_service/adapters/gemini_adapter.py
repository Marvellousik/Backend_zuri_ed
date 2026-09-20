"""
Google Gemini Provider Adapter for LexiAssist AI Infrastructure.
Supports Gemini 2.5 Flash Lite, Flash, Pro with token usage, streaming, JSON mode, and cost calculation.
"""
import os
import json
import asyncio
import logging
try:
    import google.generativeai as genai
    from google.api_core.exceptions import ResourceExhausted, GoogleAPICallError
except ImportError:
    genai = None
    ResourceExhausted = Exception
    GoogleAPICallError = Exception

from ai_service.adapters.base import (
    BaseProviderAdapter,
    GenerationResponse,
    StreamChunk,
)

logger = logging.getLogger(__name__)

# Standard pricing table (USD per 1K tokens)
GEMINI_PRICING = {
    "gemini-2.5-flash-lite": {"input": 0.00010, "output": 0.00040},
    "gemini-2.5-flash":      {"input": 0.00030, "output": 0.00250},
    "gemini-2.5-pro":        {"input": 0.00125, "output": 0.01000},
    "gemini-1.5-flash":      {"input": 0.000075, "output": 0.00030},
}
DEFAULT_MODEL = os.getenv("DEFAULT_MODEL", "gemini-2.5-flash-lite")


class GeminiAdapter(BaseProviderAdapter):
    """Adapter for Google Gemini AI models."""

    def __init__(self, api_key: Optional[str] = None):
        self.api_key = api_key or os.getenv("GEMINI_API_KEY") or os.getenv("GOOGLE_API_KEY")
        if self.api_key:
            genai.configure(api_key=self.api_key)
        else:
            logger.warning("⚠️ GEMINI_API_KEY is not configured in environment variables.")
        self._model_cache: Dict[str, genai.GenerativeModel] = {}

    def _get_model(self, model_name: str, system_instruction: Optional[str] = None) -> genai.GenerativeModel:
        cache_key = f"{model_name}:{system_instruction or ''}"
        if cache_key not in self._model_cache:
            kwargs = {"model_name": model_name}
            if system_instruction:
                kwargs["system_instruction"] = system_instruction
            self._model_cache[cache_key] = genai.GenerativeModel(**kwargs)
        return self._model_cache[cache_key]

    @staticmethod
    def calculate_cost(model_name: str, input_tokens: int, output_tokens: int) -> float:
        pricing = GEMINI_PRICING.get(model_name, GEMINI_PRICING["gemini-2.5-flash"])
        input_cost = (input_tokens / 1000.0) * pricing["input"]
        output_cost = (output_tokens / 1000.0) * pricing["output"]
        return round(input_cost + output_cost, 8)

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
        """Execute synchronous content generation with Gemini."""
        target_model = model_name or DEFAULT_MODEL
        model = self._get_model(target_model, system_instruction)

        gen_config = genai.types.GenerationConfig(
            temperature=temperature,
            max_output_tokens=max_output_tokens,
            response_mime_type=response_mime_type,
        )

        max_retries = 3
        base_delay = 1.5

        for attempt in range(max_retries):
            try:
                # Run the blocking Gemini call in an async executor
                loop = asyncio.get_event_loop()
                response = await loop.run_in_executor(
                    None,
                    lambda: model.generate_content(prompt, generation_config=gen_config)
                )
                break
            except ResourceExhausted as e:
                if attempt < max_retries - 1:
                    delay = base_delay * (2 ** attempt)
                    logger.warning(f"⏳ Gemini Rate limit (429). Retrying in {delay}s... (attempt {attempt + 1}/{max_retries})")
                    await asyncio.sleep(delay)
                else:
                    raise RuntimeError(f"Gemini API rate limit exceeded after {max_retries} attempts: {e}")
            except Exception as e:
                logger.error(f"❌ Gemini generation error: {e}")
                raise e

        # Extract tokens and usage
        input_tokens = 0
        output_tokens = 0
        if hasattr(response, "usage_metadata") and response.usage_metadata:
            input_tokens = getattr(response.usage_metadata, "prompt_token_count", 0)
            output_tokens = getattr(response.usage_metadata, "candidates_token_count", 0)
        else:
            # Approximate token counts if usage metadata is missing (4 chars ≈ 1 token)
            input_tokens = len(prompt) // 4
            output_tokens = len(response.text or "") // 4

        total_tokens = input_tokens + output_tokens
        cost = self.calculate_cost(target_model, input_tokens, output_tokens)

        # Parse JSON if requested
        parsed_json = None
        if response_mime_type == "application/json" or response.text.strip().startswith("{") or response.text.strip().startswith("["):
            try:
                text_content = response.text.strip()
                if text_content.startswith("```json"):
                    text_content = text_content.split("```json", 1)[1].rsplit("```", 1)[0].strip()
                elif text_content.startswith("```"):
                    text_content = text_content.split("```", 1)[1].rsplit("```", 1)[0].strip()
                parsed_json = json.loads(text_content)
            except Exception:
                parsed_json = None

        return GenerationResponse(
            content=response.text or "",
            parsed_json=parsed_json,
            input_tokens=input_tokens,
            output_tokens=output_tokens,
            total_tokens=total_tokens,
            cost_usd=cost,
            model_used=target_model,
            provider_used="google",
            raw_response=response,
        )

    async def generate_stream(
        self,
        prompt: str,
        system_instruction: Optional[str] = None,
        model_name: Optional[str] = None,
        temperature: float = 0.4,
        max_output_tokens: Optional[int] = None,
    ) -> AsyncGenerator[StreamChunk, None]:
        """Stream response chunks from Gemini."""
        target_model = model_name or DEFAULT_MODEL
        model = self._get_model(target_model, system_instruction)

        gen_config = genai.types.GenerationConfig(
            temperature=temperature,
            max_output_tokens=max_output_tokens,
        )

        loop = asyncio.get_event_loop()
        response = await loop.run_in_executor(
            None,
            lambda: model.generate_content(prompt, generation_config=gen_config, stream=True)
        )

        total_chars = 0
        for chunk in response:
            if chunk.text:
                total_chars += len(chunk.text)
                yield StreamChunk(
                    token=chunk.text,
                    is_final=False,
                    model_used=target_model,
                )
                await asyncio.sleep(0.005)

        # Final completion chunk
        input_tokens = 0
        output_tokens = 0
        if hasattr(response, "usage_metadata") and response.usage_metadata:
            input_tokens = getattr(response.usage_metadata, "prompt_token_count", 0)
            output_tokens = getattr(response.usage_metadata, "candidates_token_count", 0)
        else:
            input_tokens = len(prompt) // 4
            output_tokens = total_chars // 4

        cost = self.calculate_cost(target_model, input_tokens, output_tokens)

        yield StreamChunk(
            token="",
            is_final=True,
            input_tokens=input_tokens,
            output_tokens=output_tokens,
            cost_usd=cost,
            model_used=target_model,
            finish_reason="stop",
        )

    async def health(self) -> Dict[str, Any]:
        return {
            "provider": "google",
            "configured": bool(self.api_key),
            "default_model": DEFAULT_MODEL,
            "models": list(GEMINI_PRICING.keys()),
        }

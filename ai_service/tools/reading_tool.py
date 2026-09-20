"""
Reading Assistant Tool for LexiAssist AI Infrastructure.
Generates concise/detailed summaries, key concept breakdowns, and vocabulary definitions.
"""
from typing import Dict, Any, Tuple
from ai_service.tools.base import BaseTool
from ai_service.contracts.context import AIRequestContext
from ai_service.contracts.usage import TokenUsage
from ai_service.adapters.base import ModelCapability, ModelRequirements


class ReadingTool(BaseTool):
    """Handles reading.summarize and reading.analyse operations."""

    @property
    def operation_prefix(self) -> str:
        return "reading"

    async def execute(
        self,
        ctx: AIRequestContext,
        gateway: Any,
    ) -> Tuple[Dict[str, Any], TokenUsage, Dict[str, str]]:
        document_text = ctx.input.get("document_text", "")
        summary_type = ctx.parameters.get("summary_type", "concise")
        generate_tts = ctx.parameters.get("tts", False)
        voice = ctx.parameters.get("voice", "default")

        prompt = f"""You are an academic reading engine analyzing study materials.

DOCUMENT CONTENT:
{document_text[:12000]}

Analyze this document and produce a JSON response with the following format:
{{
    "summary": "...",
    "key_takeaways": ["point 1", "point 2", "point 3"],
    "vocab_terms": [
        {{"term": "...", "definition": "...", "context_snippet": "..."}}
    ]
}}

Summary style: {summary_type}. Focus on high-yield academic retention."""

        req = ModelRequirements(
            capability=ModelCapability.STRUCTURED_GENERATION,
            json_output=True,
            cost_sensitive=True,
        )
        model_name = gateway.model_router.resolve_model(req, estimated_input_chars=len(prompt))

        response = await gateway.gemini_adapter.generate(
            prompt=prompt,
            model_name=model_name,
            response_mime_type="application/json",
        )

        parsed = response.parsed_json or {}
        summary_text = parsed.get("summary", response.content)
        vocab_terms = parsed.get("vocab_terms", [])
        key_takeaways = parsed.get("key_takeaways", [])

        # Optional TTS generation via audio adapter
        tts_audio_b64 = None
        if generate_tts and summary_text:
            tts_res = await gateway.audio_adapter.synthesize_speech(summary_text[:1000], voice=voice)
            tts_audio_b64 = tts_res.get("audio_b64")

        result = {
            "summary": summary_text,
            "summary_type": summary_type,
            "key_takeaways": key_takeaways,
            "vocab_terms": vocab_terms,
            "tts_audio_b64": tts_audio_b64,
        }

        usage = TokenUsage(
            input_tokens=response.input_tokens,
            output_tokens=response.output_tokens,
            total_tokens=response.total_tokens,
            estimated_cost_usd=response.cost_usd,
        )

        model_info = {"provider": "google", "model": model_name}
        return result, usage, model_info

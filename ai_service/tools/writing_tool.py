"""
Writing Assistant Tool for LexiAssist AI Infrastructure.
Converts raw transcripts and lecture notes into well-organized academic markdown notes.
"""
from typing import Dict, Any, Tuple
from ai_service.tools.base import BaseTool
from ai_service.contracts.context import AIRequestContext
from ai_service.contracts.usage import TokenUsage
from ai_service.adapters.base import ModelCapability, ModelRequirements


class WritingTool(BaseTool):
    """Handles writing.notes and writing.transcribe operations."""

    @property
    def operation_prefix(self) -> str:
        return "writing"

    async def execute(
        self,
        ctx: AIRequestContext,
        gateway: Any,
    ) -> Tuple[Dict[str, Any], TokenUsage, Dict[str, str]]:
        operation = ctx.operation
        
        if "notes" in operation:
            raw_text = ctx.input.get("raw_text", "")
            subject = ctx.parameters.get("subject", "General")

            prompt = f"""You are an expert academic note-taker. Convert this raw transcript into structured markdown notes.

SUBJECT: {subject}

RAW TRANSCRIPT:
{raw_text[:15000]}

Formatting rules:
- ## for major topics, ### for subtopics
- Bullet points for key ideas and supporting arguments
- **Bold** all definitions and vital concepts
- > Blockquote examples or teacher analogies
- Clean up filler words while preserving 100% of the academic facts"""

            req = ModelRequirements(
                capability=ModelCapability.STRUCTURED_GENERATION,
                cost_sensitive=True,
            )
            model_name = gateway.model_router.resolve_model(req, estimated_input_chars=len(prompt))

            response = await gateway.gemini_adapter.generate(
                prompt=prompt,
                model_name=model_name,
                temperature=0.2,
            )

            result = {
                "structured_notes": response.content,
                "subject": subject,
            }

            usage = TokenUsage(
                input_tokens=response.input_tokens,
                output_tokens=response.output_tokens,
                total_tokens=response.total_tokens,
                estimated_cost_usd=response.cost_usd,
            )

            return result, usage, {"provider": "google", "model": model_name}

        elif "transcribe" in operation:
            # Audio transcription
            audio_bytes = ctx.input.get("audio_bytes", b"")
            filename = ctx.input.get("filename", "audio.wav")
            
            stt_res = await gateway.audio_adapter.transcribe_audio(audio_bytes, filename=filename)
            
            return {
                "transcript": stt_res.get("text", ""),
                "language": stt_res.get("language", "en-US"),
                "confidence": stt_res.get("confidence", 0.0),
            }, TokenUsage(), {"provider": "audio", "model": "stt"}

        else:
            raise ValueError(f"Unknown writing operation: {operation}")

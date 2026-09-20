"""
Study Buddy Tool for LexiAssist AI Infrastructure.
Generates structured flashcards, multiple-choice quizzes, and theory questions.
"""
from typing import Dict, Any, Tuple
from ai_service.tools.base import BaseTool
from ai_service.contracts.context import AIRequestContext
from ai_service.contracts.usage import TokenUsage
from ai_service.adapters.base import ModelCapability, ModelRequirements


class StudyTool(BaseTool):
    """Handles study.flashcards.generate and study.quiz.generate operations."""

    @property
    def operation_prefix(self) -> str:
        return "study"

    async def execute(
        self,
        ctx: AIRequestContext,
        gateway: Any,
    ) -> Tuple[Dict[str, Any], TokenUsage, Dict[str, str]]:
        operation = ctx.operation
        document_text = ctx.input.get("document_text", ctx.input.get("query", ""))
        
        if "flashcards" in operation:
            return await self._generate_flashcards(ctx, gateway, document_text)
        elif "quiz" in operation:
            return await self._generate_quiz(ctx, gateway, document_text)
        else:
            raise ValueError(f"Unknown study operation: {operation}")

    async def _generate_flashcards(
        self, ctx: AIRequestContext, gateway: Any, text: str
    ) -> Tuple[Dict[str, Any], TokenUsage, Dict[str, str]]:
        count = ctx.parameters.get("count", 10)
        
        prompt = f"""You are an educational AI creating study flashcards.

SOURCE MATERIAL:
{text[:12000]}

Generate exactly {count} high-yield study flashcards from the text.
Format strictly as JSON:
{{
    "flashcards": [
        {{
            "front": "Clear, specific question or concept",
            "back": "Concise, complete explanation or definition",
            "topic": "Subject/Topic tag"
        }}
    ]
}}"""

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
        flashcards = parsed.get("flashcards", [])

        result = {
            "num_requested": count,
            "num_generated": len(flashcards),
            "flashcards": flashcards,
        }

        usage = TokenUsage(
            input_tokens=response.input_tokens,
            output_tokens=response.output_tokens,
            total_tokens=response.total_tokens,
            estimated_cost_usd=response.cost_usd,
        )

        return result, usage, {"provider": "google", "model": model_name}

    async def _generate_quiz(
        self, ctx: AIRequestContext, gateway: Any, text: str
    ) -> Tuple[Dict[str, Any], TokenUsage, Dict[str, str]]:
        count = ctx.parameters.get("count", 5)
        quiz_type = ctx.parameters.get("quiz_type", "multiple_choice")

        if quiz_type == "multiple_choice":
            format_spec = """{
    "questions": [
        {
            "question": "Question text",
            "options": {"A": "Option A", "B": "Option B", "C": "Option C", "D": "Option D"},
            "correct_answer": "A",
            "explanation": "Why this answer is correct",
            "topic": "Topic tag"
        }
    ]
}"""
        else:
            format_spec = """{
    "questions": [
        {
            "question": "Theory/essay question text",
            "model_answer": "Comprehensive model answer",
            "marking_guide": ["1 mark for...", "2 marks for..."],
            "marks": 5,
            "topic": "Topic tag"
        }
    ]
}"""

        prompt = f"""You are an educational assessment AI.

SOURCE MATERIAL:
{text[:12000]}

Generate {count} {quiz_type} questions based strictly on the source material.
Format strictly as JSON:
{format_spec}"""

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
        questions = parsed.get("questions", [])

        result = {
            "quiz_type": quiz_type,
            "num_requested": count,
            "num_generated": len(questions),
            "questions": questions,
        }

        usage = TokenUsage(
            input_tokens=response.input_tokens,
            output_tokens=response.output_tokens,
            total_tokens=response.total_tokens,
            estimated_cost_usd=response.cost_usd,
        )

        return result, usage, {"provider": "google", "model": model_name}

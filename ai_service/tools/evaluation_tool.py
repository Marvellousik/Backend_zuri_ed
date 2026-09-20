"""
Evaluation and Grading Tool for LexiAssist AI Infrastructure.
Automates quiz grading, answer key validation, and personalized learning feedback.
"""
from typing import Dict, Any, Tuple
from ai_service.tools.base import BaseTool
from ai_service.contracts.context import AIRequestContext
from ai_service.contracts.usage import TokenUsage
from ai_service.storage.database import get_db_session
from ai_service.storage.models import QuizAnswerKey


class EvaluationTool(BaseTool):
    """Handles evaluation.grade and evaluation.feedback operations."""

    @property
    def operation_prefix(self) -> str:
        return "evaluation"

    async def execute(
        self,
        ctx: AIRequestContext,
        gateway: Any,
    ) -> Tuple[Dict[str, Any], TokenUsage, Dict[str, str]]:
        operation = ctx.operation
        
        if "grade" in operation:
            quiz_id = ctx.input.get("quiz_id")
            user_answers = ctx.input.get("answers", {})

            # Lookup answer key if stored
            answer_key = {}
            with get_db_session() as session:
                key_obj = session.query(QuizAnswerKey).filter(QuizAnswerKey.quiz_id == quiz_id).first()
                if key_obj:
                    answer_key = key_obj.answers

            # Grade matching answers
            correct_count = 0
            feedback = {}
            for q_id, expected in answer_key.items():
                actual = user_answers.get(q_id)
                if str(actual).strip().lower() == str(expected).strip().lower():
                    correct_count += 1
                    feedback[q_id] = "Correct!"
                else:
                    feedback[q_id] = f"Incorrect. Correct answer was {expected}."

            total = len(answer_key) or len(user_answers) or 1
            score = round((correct_count / total) * 100, 2)

            result = {
                "quiz_id": quiz_id,
                "score_percent": score,
                "correct_count": correct_count,
                "total_questions": total,
                "feedback": feedback,
            }

            return result, TokenUsage(), {"provider": "internal", "model": "rule_grader"}

        else:
            raise ValueError(f"Unsupported evaluation operation: {operation}")

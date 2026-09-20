"""
Contextual Assessment & Adaptive Practice Engine for Lexi (Phase 31).
Generates targeted, difficulty-calibrated practice sessions addressing active student
learning gaps and weak concept areas. Evaluates attempts with latency/confidence telemetry,
feeds the SignalIngestionService, and dynamically manages gap resolution.
"""
import uuid
import logging
from datetime import datetime, timezone
from typing import Dict, Any, List, Optional
from sqlalchemy.orm import Session

from academic_service.models.orm import (
    Course,
    StudentKnowledgeState,
    LearningGap,
)
from academic_service.models.schema import LearningSignalCreate
from academic_service.services.signal_ingestion_service import SignalIngestionService

logger = logging.getLogger(__name__)


class AdaptivePracticeService:
    """
    Adaptive contextual practice and assessment generator.
    """

    @classmethod
    def calculate_difficulty_level(cls, mastery_score: int) -> int:
        """
        Maps student mastery score (0-100) to pedagogical difficulty tier (1-5):
        - Tier 1 (0-29%): Foundational recall, term recognition, basic definitions.
        - Tier 2 (30-49%): Basic mechanics, simple trace, state identification.
        - Tier 3 (50-69%): Standard application, recurrence formulation, intermediate logic.
        - Tier 4 (70-84%): Complex problem formulation, edge-case analysis.
        - Tier 5 (85-100%): Advanced synthesis, proof sketches, optimization trade-offs.
        """
        if mastery_score < 30:
            return 1
        elif mastery_score < 50:
            return 2
        elif mastery_score < 70:
            return 3
        elif mastery_score < 85:
            return 4
        else:
            return 5

    @classmethod
    def generate_adaptive_practice_session(
        cls,
        db: Session,
        user_id: str,
        course_id: str,
        gap_id: Optional[str] = None,
    ) -> Dict[str, Any]:
        """
        Generates an adaptive practice session targeting active learning gaps or weakest topics.
        Generates calibrated MCQ and structured theory questions with marking rubrics.
        """
        course = (
            db.query(Course)
            .filter((Course.id == course_id) | (Course.code.ilike(course_id.strip())))
            .first()
        )
        if not course:
            raise ValueError(f"Course '{course_id}' not found.")

        target_gap: Optional[LearningGap] = None
        target_topic: str = "General"
        target_subtopic: str = "Core Concepts"
        current_mastery = 30

        # 1. Target by specific gap_id if provided
        if gap_id:
            target_gap = (
                db.query(LearningGap)
                .filter(LearningGap.id == gap_id, LearningGap.user_id == user_id)
                .first()
            )

        # 2. Or find the most severe active learning gap
        if not target_gap:
            target_gap = (
                db.query(LearningGap)
                .filter(
                    LearningGap.user_id == user_id,
                    LearningGap.course_id == course.id,
                    LearningGap.status == "active",
                )
                .order_by(LearningGap.created_at.desc())
                .first()
            )

        if target_gap:
            target_topic = target_gap.topic
            target_subtopic = target_gap.subtopic

            # Fetch corresponding knowledge state
            ks = (
                db.query(StudentKnowledgeState)
                .filter(
                    StudentKnowledgeState.user_id == user_id,
                    StudentKnowledgeState.course_id == course.id,
                    StudentKnowledgeState.subtopic == target_subtopic,
                )
                .first()
            )
            if ks:
                current_mastery = ks.mastery_score
        else:
            # 3. Fallback to weakest topic in knowledge state
            weakest_ks = (
                db.query(StudentKnowledgeState)
                .filter(
                    StudentKnowledgeState.user_id == user_id,
                    StudentKnowledgeState.course_id == course.id,
                )
                .order_by(StudentKnowledgeState.mastery_score.asc())
                .first()
            )
            if weakest_ks:
                target_topic = weakest_ks.topic
                target_subtopic = weakest_ks.subtopic
                current_mastery = weakest_ks.mastery_score
            elif course.syllabus:
                first_mod = course.syllabus[0]
                target_topic = first_mod.get("topic", "Introduction")
                subtopics = first_mod.get("subtopics", [target_topic])
                target_subtopic = subtopics[0] if subtopics else target_topic

        difficulty = cls.calculate_difficulty_level(current_mastery)
        session_id = f"prac_{uuid.uuid4()}"

        # 4. Synthesize difficulty-calibrated questions
        questions: List[Dict[str, Any]] = []

        # Question 1: Multiple Choice Question (Foundational/Mechanics)
        questions.append({
            "question_id": f"q_{session_id}_1",
            "type": "multiple_choice",
            "difficulty": difficulty,
            "topic": target_topic,
            "subtopic": target_subtopic,
            "question_text": f"In {target_subtopic}, which of the following is the primary defining requirement?",
            "options": [
                f"A) Subproblems must be optimal and overlap during recursive decomposition",
                f"B) Subproblems must execute in linear amortized time",
                f"C) Greedy selections can be made without recursive validation",
                f"D) The state space must be non-deterministic",
            ],
            "correct_option": "A",
            "explanation": f"Core requirement of {target_subtopic}: requires optimal subproblem decomposition.",
            "rubric": "1 point for selecting option A.",
            "points": 1,
        })

        # Question 2: Diagnostic Application Question (MCQ)
        questions.append({
            "question_id": f"q_{session_id}_2",
            "type": "multiple_choice",
            "difficulty": min(5, difficulty + 1),
            "topic": target_topic,
            "subtopic": target_subtopic,
            "question_text": f"When solving a problem with {target_subtopic}, what happens if base cases are omitted in the recurrence?",
            "options": [
                f"A) The recursion terminates prematurely with null state",
                f"B) The call stack experiences unbounded recursion or infinite loop",
                f"C) Tabulation completes with optimal runtime regardless",
                f"D) Space complexity reduces to O(1)",
            ],
            "correct_option": "B",
            "explanation": "Omitting base cases prevents recursive termination, causing infinite call stack recursion.",
            "rubric": "1 point for selecting option B.",
            "points": 1,
        })

        # Question 3: Structured Theory Question with Multi-Criteria Rubric
        theory_rubric = {
            "criteria": [
                {"criterion": "Identifies recursive state representation", "points": 2},
                {"criterion": "Formulates boundary / base conditions", "points": 1},
                {"criterion": "Defines transition equation / recurrence logic", "points": 2},
            ],
            "total_points": 5,
        }

        questions.append({
            "question_id": f"q_{session_id}_3",
            "type": "theory_rubric",
            "difficulty": difficulty,
            "topic": target_topic,
            "subtopic": target_subtopic,
            "question_text": (
                f"For the concept of {target_subtopic}: Describe the formal recurrence relation, "
                f"explicitly stating the base cases and state variables."
            ),
            "sample_solution": (
                f"State representation: S(i, j) defines the optimal value at step i with capacity j. "
                f"Base condition: S(0, j) = 0 and S(i, 0) = 0. "
                f"Transition: S(i, j) = max(S(i-1, j), S(i-1, j-w_i) + v_i)."
            ),
            "rubric": theory_rubric,
            "points": 5,
        })

        return {
            "session_id": session_id,
            "user_id": user_id,
            "course_id": course.id,
            "course_code": course.code,
            "target_gap_id": target_gap.id if target_gap else None,
            "target_topic": target_topic,
            "target_subtopic": target_subtopic,
            "student_current_mastery": current_mastery,
            "difficulty_level": difficulty,
            "total_questions": len(questions),
            "max_points": sum(q["points"] for q in questions),
            "questions": questions,
            "created_at": datetime.now(timezone.utc).isoformat(),
        }

    @classmethod
    def submit_practice_attempt(
        cls,
        db: Session,
        user_id: str,
        course_id: str,
        attempt_data: Dict[str, Any],
    ) -> Dict[str, Any]:
        """
        Grades an adaptive practice attempt, extracts latency and confidence telemetry,
        updates the student's mastery score via SignalIngestionService, and resolves active
        learning gaps upon demonstrated mastery.
        """
        session_id = attempt_data.get("session_id", f"prac_{uuid.uuid4()}")
        topic = attempt_data.get("topic", "General")
        subtopic = attempt_data.get("subtopic", "Core Concepts")
        gap_id = attempt_data.get("gap_id")
        answers = attempt_data.get("answers", {})

        # Telemetry
        latency_ms = attempt_data.get("latency_ms", 12000)
        confidence_level = attempt_data.get("confidence_level", 3)

        # 1. Scoring logic
        # Default session has 7 total points (1 + 1 + 5)
        points_earned = 0
        total_possible = 0

        feedback_items: List[Dict[str, Any]] = []

        # Question 1: MCQ (1 pt)
        ans1 = str(answers.get("q_1", answers.get(f"q_{session_id}_1", ""))).strip().upper()
        total_possible += 1
        if ans1 == "A" or "OPTIMAL" in ans1:
            points_earned += 1
            feedback_items.append({"question_id": 1, "status": "correct", "feedback": "Correct definition identified."})
        else:
            feedback_items.append({"question_id": 1, "status": "incorrect", "feedback": "Review optimal substructure definition."})

        # Question 2: MCQ (1 pt)
        ans2 = str(answers.get("q_2", answers.get(f"q_{session_id}_2", ""))).strip().upper()
        total_possible += 1
        if ans2 == "B" or "INFINITE" in ans2 or "STACK" in ans2:
            points_earned += 1
            feedback_items.append({"question_id": 2, "status": "correct", "feedback": "Correct base case necessity recognized."})
        else:
            feedback_items.append({"question_id": 2, "status": "incorrect", "feedback": "Base cases prevent unbounded recursion."})

        # Question 3: Theory rubric (up to 5 pts)
        theory_ans = str(answers.get("q_3", answers.get(f"q_{session_id}_3", ""))).lower()
        total_possible += 5
        theory_score = 0

        if "state" in theory_ans or "representation" in theory_ans:
            theory_score += 2
        if "base" in theory_ans or "0" in theory_ans:
            theory_score += 1
        if "transition" in theory_ans or "max" in theory_ans or "recurrence" in theory_ans or "subproblem" in theory_ans:
            theory_score += 2

        # In case explicit theory points were passed by an evaluator
        if "theory_points" in attempt_data:
            theory_score = int(attempt_data["theory_points"])

        points_earned += theory_score
        feedback_items.append({
            "question_id": 3,
            "status": "partial" if theory_score < 5 else "correct",
            "score": theory_score,
            "max_score": 5,
            "feedback": f"Theory rubric evaluation awarded {theory_score}/5 points.",
        })

        percentage = int(round((points_earned / max(1, total_possible)) * 100))

        # 2. Ingest learning signal with telemetry
        signal = LearningSignalCreate(
            course_id=course_id,
            topic=topic,
            subtopic=subtopic,
            signal_type="adaptive_practice",
            score=points_earned,
            max_score=total_possible,
            latency_ms=latency_ms,
            confidence_level=confidence_level,
            details={
                "session_id": session_id,
                "gap_id": gap_id,
                "percentage": percentage,
                "feedback": feedback_items,
            },
        )

        knowledge_dto = SignalIngestionService.record_signal(
            db=db,
            user_id=user_id,
            signal_data=signal,
        )

        # 3. Check if LearningGap was resolved
        gap_resolved = False
        if gap_id:
            gap = db.query(LearningGap).filter(LearningGap.id == gap_id).first()
            if gap and gap.status == "resolved":
                gap_resolved = True
        elif knowledge_dto.mastery_score >= 75:
            gap_resolved = True

        return {
            "session_id": session_id,
            "user_id": user_id,
            "course_id": course_id,
            "topic": topic,
            "subtopic": subtopic,
            "points_earned": points_earned,
            "total_possible": total_possible,
            "percentage": percentage,
            "updated_mastery_score": knowledge_dto.mastery_score,
            "status": knowledge_dto.status,
            "gap_resolved": gap_resolved,
            "feedback_items": feedback_items,
            "evaluated_at": datetime.now(timezone.utc).isoformat(),
        }

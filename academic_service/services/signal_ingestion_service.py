"""
Learning Signal Ingestion Service for Lexi (Phase 26: Learning Signal Intelligence).
Processes atomic student interactions (quizzes, flashcard flips, diagnostics, chat responses),
incorporating response latency (ms) and confidence level (1-5 scale) weighting to dynamically
update student knowledge states and detect/update active learning gaps.
"""
import uuid
import logging
from datetime import datetime, timezone
from typing import Optional, Dict, Any, Tuple
from sqlalchemy.orm import Session

from academic_service.models.orm import (
    Course,
    StudentKnowledgeState,
    LearningSignal,
    LearningGap,
)
from academic_service.models.schema import LearningSignalCreate, KnowledgeStateDTO

logger = logging.getLogger(__name__)

# Latency Thresholds (in milliseconds)
FAST_LATENCY_THRESHOLD_MS = 6000    # < 6s: automaticity & fluency bonus
SLOW_LATENCY_THRESHOLD_MS = 20000   # > 20s: cognitive friction / hesitation


class SignalIngestionService:
    """
    Ingests learning events with latency and confidence telemetry,
    calculates mastery progression via adaptive exponential smoothing,
    and dynamically detects weak concept areas and learning gaps.
    """

    @classmethod
    def compute_signal_weighting(
        cls,
        base_score_pct: float,
        latency_ms: Optional[int] = None,
        confidence_level: Optional[int] = None,
    ) -> Tuple[int, float, bool, bool]:
        """
        Computes the adjusted score percentage, EMA alpha, and behavioral flags:
        - Latency multiplier:
            * Fast correct (< 6s): bonus for fluent automaticity (1.15x).
            * Slow correct (> 20s): tentative mastery / guessed (0.85x).
            * Slow incorrect (> 20s): deep cognitive gap flag.
        - Confidence weighting (1-5 scale):
            * Confident (4-5) & Incorrect: Dunning-Kruger misconception trap!
            * Low confidence (1-2) & Correct: lucky guess dampener.
            * Confident (4-5) & Correct: validated mastery.
        Returns (adjusted_pct, alpha_weight, hesitation_flag, misconception_trap_flag).
        """
        is_success = base_score_pct >= 70.0
        latency_mult = 1.0
        hesitation = False

        if latency_ms is not None:
            if is_success:
                if latency_ms < FAST_LATENCY_THRESHOLD_MS:
                    latency_mult = 1.15
                elif latency_ms > SLOW_LATENCY_THRESHOLD_MS:
                    latency_mult = 0.85
                    hesitation = True
            else:
                if latency_ms > SLOW_LATENCY_THRESHOLD_MS:
                    hesitation = True

        confidence_mult = 1.0
        misconception_trap = False
        alpha = 0.40  # Standard EMA weight

        if confidence_level is not None:
            if is_success:
                if confidence_level in (4, 5):
                    confidence_mult = 1.10
                    alpha = 0.45
                elif confidence_level in (1, 2):
                    confidence_mult = 0.80  # Lucky guess dampening
                    alpha = 0.20
            else:
                if confidence_level in (4, 5):
                    misconception_trap = True  # Confidently wrong
                    alpha = 0.50

        raw_adjusted = base_score_pct * latency_mult * confidence_mult
        adjusted_pct = int(round(max(0.0, min(100.0, raw_adjusted))))
        return adjusted_pct, alpha, hesitation, misconception_trap

    @classmethod
    def record_signal(
        cls,
        db: Session,
        user_id: str,
        signal_data: LearningSignalCreate,
    ) -> KnowledgeStateDTO:
        """
        Record a learning event, apply latency and confidence weighting,
        update student knowledge state, and manage learning gaps.
        """
        course = (
            db.query(Course)
            .filter((Course.id == signal_data.course_id) | (Course.code.ilike(signal_data.course_id.strip())))
            .first()
        )
        if not course:
            raise ValueError(f"Course '{signal_data.course_id}' not found.")

        # Extract telemetry
        latency_ms = getattr(signal_data, "latency_ms", None)
        confidence_level = getattr(signal_data, "confidence_level", None)
        details = signal_data.details or {}

        if latency_ms is None and "latency_ms" in details:
            latency_ms = details["latency_ms"]
        if confidence_level is None and "confidence_level" in details:
            confidence_level = details["confidence_level"]

        # 1. Record Atomic Learning Signal
        sig = LearningSignal(
            id=str(uuid.uuid4()),
            user_id=user_id,
            course_id=course.id,
            topic=signal_data.topic,
            subtopic=signal_data.subtopic,
            signal_type=signal_data.signal_type,
            score=signal_data.score,
            max_score=signal_data.max_score,
            latency_ms=latency_ms,
            confidence_level=confidence_level,
            raw_details=details,
        )
        db.add(sig)

        # 2. Find or create StudentKnowledgeState
        state = (
            db.query(StudentKnowledgeState)
            .filter(
                StudentKnowledgeState.user_id == user_id,
                StudentKnowledgeState.course_id == course.id,
                StudentKnowledgeState.topic == signal_data.topic,
                StudentKnowledgeState.subtopic == signal_data.subtopic,
            )
            .first()
        )

        if not state:
            state = StudentKnowledgeState(
                id=str(uuid.uuid4()),
                user_id=user_id,
                course_id=course.id,
                topic=signal_data.topic,
                subtopic=signal_data.subtopic,
                mastery_score=0,
                status="not_started",
                total_attempts=0,
                correct_attempts=0,
            )
            db.add(state)

        # 3. Calculate Base Performance & Telemetry Weighting
        base_attempt_pct = (signal_data.score / max(1, signal_data.max_score)) * 100.0
        is_success = base_attempt_pct >= 70.0

        state.total_attempts += 1
        if is_success:
            state.correct_attempts += 1

        adjusted_pct, alpha, hesitation, misconception_trap = cls.compute_signal_weighting(
            base_score_pct=base_attempt_pct,
            latency_ms=latency_ms,
            confidence_level=confidence_level,
        )

        # 4. Adaptive Exponential Moving Average update
        if state.total_attempts == 1:
            state.mastery_score = adjusted_pct
        else:
            state.mastery_score = int(round((1.0 - alpha) * state.mastery_score + alpha * adjusted_pct))

        state.mastery_score = max(0, min(100, state.mastery_score))
        state.last_evaluated_at = datetime.now(timezone.utc)

        # Extract or construct error summary
        if details and "error_summary" in details:
            state.last_error_summary = details["error_summary"]
        elif not is_success:
            err_msg = f"Missed question on {signal_data.subtopic} ({signal_data.score}/{signal_data.max_score})"
            if misconception_trap:
                err_msg += " with high confidence (entrenched misconception)."
            elif hesitation:
                err_msg += " with prolonged hesitation."
            state.last_error_summary = err_msg

        # 5. Status Transition
        if state.mastery_score >= 80 and not misconception_trap:
            state.status = "mastered"
        elif state.mastery_score >= 50:
            state.status = "learning"
        else:
            state.status = "struggling"

        # 6. Dynamic Learning Gap Detection & Updating
        # Triggers when mastery < 50% or when significant error/hesitation patterns are detected
        needs_gap = (
            state.mastery_score < 50
            or state.status == "struggling"
            or misconception_trap
            or (not is_success and hesitation)
        )

        existing_gap = (
            db.query(LearningGap)
            .filter(
                LearningGap.user_id == user_id,
                LearningGap.course_id == course.id,
                LearningGap.topic == signal_data.topic,
                LearningGap.subtopic == signal_data.subtopic,
                LearningGap.status == "active",
            )
            .first()
        )

        if needs_gap:
            # Determine severity based on telemetry
            if state.mastery_score < 35 or misconception_trap:
                severity = "high"
            elif state.mastery_score < 50 or hesitation:
                severity = "medium"
            else:
                severity = "low"

            description = state.last_error_summary or f"Struggling with {signal_data.subtopic} concepts."
            if misconception_trap:
                description = f"[Entrenched Misconception] {description}"
            elif hesitation:
                description = f"[Hesitation / Slow Response] {description}"

            if existing_gap:
                existing_gap.severity = severity
                existing_gap.gap_description = description
            else:
                gap = LearningGap(
                    id=str(uuid.uuid4()),
                    user_id=user_id,
                    course_id=course.id,
                    topic=signal_data.topic,
                    subtopic=signal_data.subtopic,
                    gap_description=description,
                    severity=severity,
                    status="active",
                )
                db.add(gap)
        elif state.mastery_score >= 75 and is_success and not hesitation:
            # Resolve active gap upon demonstrated mastery
            if existing_gap:
                existing_gap.status = "resolved"
                existing_gap.resolved_at = datetime.now(timezone.utc)

        db.commit()
        logger.info(
            f"🎯 Signal ingested for {user_id} on {course.code} [{signal_data.subtopic}]: "
            f"Mastery: {state.mastery_score}% ({state.status}) | Latency: {latency_ms}ms | Confidence: {confidence_level}"
        )

        return KnowledgeStateDTO(
            topic=state.topic,
            subtopic=state.subtopic,
            mastery_score=state.mastery_score,
            status=state.status,
            total_attempts=state.total_attempts,
            correct_attempts=state.correct_attempts,
            last_evaluated_at=state.last_evaluated_at.isoformat() if state.last_evaluated_at else None,
            last_error_summary=state.last_error_summary,
        )

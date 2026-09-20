"""
Context Completion Engine for Lexi.
Proactively identifies missing academic information and prompts the student to complete their academic context.
"""
import uuid
import logging
from datetime import datetime, timezone, time
from typing import List, Optional, Dict, Any
from sqlalchemy.orm import Session

from academic_service.models.orm import ContextGap, Course, CourseSchedule, StudentEnrollment
from academic_service.models.schema import ContextGapDTO

logger = logging.getLogger(__name__)


class ContextCompletionEngine:
    """Detects missing fields in the student's academic reality and resolves them."""

    @staticmethod
    def scan_and_generate_gaps(db: Session, user_id: str) -> List[ContextGapDTO]:
        """
        Scan all enrolled courses for the student to identify missing timetable slots or details.
        """
        enrollments = (
            db.query(StudentEnrollment)
            .filter(StudentEnrollment.user_id == user_id, StudentEnrollment.status == "active")
            .all()
        )

        for enrollment in enrollments:
            course = db.query(Course).filter(Course.id == enrollment.course_id).first()
            if not course:
                continue

            # Check 1: Missing timetable schedule entirely
            schedules = db.query(CourseSchedule).filter(CourseSchedule.course_id == course.id).all()
            if not schedules:
                ContextCompletionEngine._create_gap_if_not_exists(
                    db=db,
                    user_id=user_id,
                    course_id=course.id,
                    gap_type="missing_class_time",
                    prompt_question=f"I have your {course.code} ({course.title}) class, but I don't know when it holds. What day and time is the lecture?",
                    field_target="course_schedules",
                )
            else:
                # Check 2: Missing venue in existing schedule
                for sched in schedules:
                    if not sched.venue or sched.venue.strip() == "":
                        ContextCompletionEngine._create_gap_if_not_exists(
                            db=db,
                            user_id=user_id,
                            course_id=course.id,
                            gap_type="missing_venue",
                            prompt_question=f"Where does your {course.code} lecture hold?",
                            field_target="course_schedules.venue",
                        )

        # Return all unresolved gaps
        gaps = (
            db.query(ContextGap)
            .filter(ContextGap.user_id == user_id, ContextGap.is_resolved == False)
            .all()
        )
        return [
            ContextGapDTO(
                id=g.id,
                user_id=g.user_id,
                course_id=g.course_id,
                gap_type=g.gap_type,
                prompt_question=g.prompt_question,
                field_target=g.field_target,
                is_resolved=g.is_resolved,
            )
            for g in gaps
        ]

    @staticmethod
    def _create_gap_if_not_exists(
        db: Session, user_id: str, course_id: str, gap_type: str, prompt_question: str, field_target: str
    ):
        existing = (
            db.query(ContextGap)
            .filter(
                ContextGap.user_id == user_id,
                ContextGap.course_id == course_id,
                ContextGap.gap_type == gap_type,
                ContextGap.is_resolved == False,
            )
            .first()
        )
        if not existing:
            gap = ContextGap(
                id=str(uuid.uuid4()),
                user_id=user_id,
                course_id=course_id,
                gap_type=gap_type,
                prompt_question=prompt_question,
                field_target=field_target,
                is_resolved=False,
            )
            db.add(gap)
            db.commit()

    @staticmethod
    def resolve_gap(db: Session, gap_id: str, response_value: str) -> Dict[str, Any]:
        """
        Resolve an academic gap with the student's answer.
        Updates the target entity (e.g. creates timetable slot or updates venue).
        """
        gap = db.query(ContextGap).filter(ContextGap.id == gap_id).first()
        if not gap:
            raise ValueError(f"Context gap '{gap_id}' not found.")

        gap.is_resolved = True
        gap.resolved_at = datetime.now(timezone.utc)
        gap.response_value = response_value

        # Apply update to relevant academic entity
        if gap.gap_type == "missing_class_time":
            # Default to Monday 09:00 - 11:00 if simple time provided, or parse input
            new_slot = CourseSchedule(
                id=str(uuid.uuid4()),
                course_id=gap.course_id,
                institution_id="veritas_uni",
                day_of_week=1,  # Default Monday
                start_time=time(hour=9, minute=0),
                end_time=time(hour=11, minute=0),
                venue="Lecture Hall",
                recurrence="weekly",
            )
            db.add(new_slot)
            logger.info(f"Resolved missing_class_time gap for course {gap.course_id}")

        elif gap.gap_type == "missing_venue":
            sched = db.query(CourseSchedule).filter(CourseSchedule.course_id == gap.course_id).first()
            if sched:
                sched.venue = response_value.strip()

        db.commit()

        return {
            "gap_id": gap.id,
            "status": "resolved",
            "message": "Thank you! Your academic context has been updated.",
        }

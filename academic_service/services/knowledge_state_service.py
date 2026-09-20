"""
Student Knowledge State Service for Lexi.
Maintains topic-level mastery graphs, syllabus initialization, and weak topic identification.
"""
import uuid
import logging
from datetime import datetime, timezone
from typing import List, Optional, Dict, Any
from sqlalchemy.orm import Session

from academic_service.models.orm import (
    Course,
    StudentKnowledgeState,
    StudentEnrollment,
    LearningGap,
)
from academic_service.models.schema import (
    KnowledgeStateDTO,
    TopicMasteryGroupDTO,
    CourseMasteryGraphDTO,
    WeakTopicDTO,
)

logger = logging.getLogger(__name__)


class KnowledgeStateService:
    """Manages student topic mastery and learning gap lookups."""

    @staticmethod
    def initialize_student_course_state(db: Session, user_id: str, course_id: str):
        """Populate initial baseline knowledge state from the course syllabus."""
        course = (
            db.query(Course)
            .filter((Course.id == course_id) | (Course.code.ilike(course_id.strip())))
            .first()
        )
        if not course or not course.syllabus:
            return

        for topic_entry in course.syllabus:
            topic_name = topic_entry.get("topic", "General")
            subtopics = topic_entry.get("subtopics", [topic_name])
            for sub in subtopics:
                existing = (
                    db.query(StudentKnowledgeState)
                    .filter(
                        StudentKnowledgeState.user_id == user_id,
                        StudentKnowledgeState.course_id == course.id,
                        StudentKnowledgeState.topic == topic_name,
                        StudentKnowledgeState.subtopic == sub,
                    )
                    .first()
                )
                if not existing:
                    ks = StudentKnowledgeState(
                        id=str(uuid.uuid4()),
                        user_id=user_id,
                        course_id=course.id,
                        topic=topic_name,
                        subtopic=sub,
                        mastery_score=0,
                        status="not_started",
                        total_attempts=0,
                        correct_attempts=0,
                    )
                    db.add(ks)
        db.commit()

    @staticmethod
    def get_course_mastery_graph(db: Session, user_id: str, course_id: str) -> Optional[CourseMasteryGraphDTO]:
        """Fetch full hierarchical mastery graph for a course."""
        course = (
            db.query(Course)
            .filter((Course.id == course_id) | (Course.code.ilike(course_id.strip())))
            .first()
        )
        if not course:
            return None

        # Auto-initialize if student has no entries yet
        existing_count = (
            db.query(StudentKnowledgeState)
            .filter(StudentKnowledgeState.user_id == user_id, StudentKnowledgeState.course_id == course.id)
            .count()
        )
        if existing_count == 0:
            KnowledgeStateService.initialize_student_course_state(db, user_id, course.id)

        states = (
            db.query(StudentKnowledgeState)
            .filter(StudentKnowledgeState.user_id == user_id, StudentKnowledgeState.course_id == course.id)
            .all()
        )

        # Group by topic
        grouped: Dict[str, List[KnowledgeStateDTO]] = {}
        for s in states:
            dto = KnowledgeStateDTO(
                topic=s.topic,
                subtopic=s.subtopic,
                mastery_score=s.mastery_score,
                status=s.status,
                total_attempts=s.total_attempts,
                correct_attempts=s.correct_attempts,
                last_evaluated_at=s.last_evaluated_at.isoformat() if s.last_evaluated_at else None,
                last_error_summary=s.last_error_summary,
            )
            grouped.setdefault(s.topic, []).append(dto)

        topic_groups = []
        all_scores = []
        mastered_count = 0
        struggling_count = 0

        for topic_name, subtopics in grouped.items():
            avg_score = int(sum(sub.mastery_score for sub in subtopics) / max(1, len(subtopics)))
            topic_groups.append(
                TopicMasteryGroupDTO(
                    topic=topic_name,
                    average_mastery=avg_score,
                    subtopics=subtopics,
                )
            )
            for sub in subtopics:
                all_scores.append(sub.mastery_score)
                if sub.status == "mastered":
                    mastered_count += 1
                elif sub.status == "struggling":
                    struggling_count += 1

        overall_mastery = int(sum(all_scores) / max(1, len(all_scores))) if all_scores else 0

        return CourseMasteryGraphDTO(
            course_id=course.id,
            course_code=course.code,
            overall_mastery=overall_mastery,
            mastered_count=mastered_count,
            struggling_count=struggling_count,
            total_subtopics=len(states),
            topic_groups=topic_groups,
        )

    @staticmethod
    def get_weakest_topics(db: Session, user_id: str, course_id: str, limit: int = 3) -> List[WeakTopicDTO]:
        """Identify top struggling topics to feed the Proactive Engine and Course Partner."""
        course = (
            db.query(Course)
            .filter((Course.id == course_id) | (Course.code.ilike(course_id.strip())))
            .first()
        )
        if not course:
            return []

        # Find topics where status is 'struggling' or lowest mastery with attempts > 0
        weak_states = (
            db.query(StudentKnowledgeState)
            .filter(
                StudentKnowledgeState.user_id == user_id,
                StudentKnowledgeState.course_id == course.id,
                StudentKnowledgeState.total_attempts > 0,
            )
            .order_by(StudentKnowledgeState.mastery_score.asc(), StudentKnowledgeState.total_attempts.desc())
            .limit(limit)
            .all()
        )

        results = []
        for s in weak_states:
            if s.mastery_score < 70:
                reason = s.last_error_summary or f"Scored {s.mastery_score}% across {s.total_attempts} recent attempts."
                results.append(
                    WeakTopicDTO(
                        course_id=course.id,
                        course_code=course.code,
                        topic=s.topic,
                        subtopic=s.subtopic,
                        mastery_score=s.mastery_score,
                        status=s.status,
                        reason=reason,
                    )
                )
        return results

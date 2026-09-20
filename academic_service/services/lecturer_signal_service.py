"""
Lecturer Signal & Pedagogical Intervention Service for Lexi (Phases 3, 32, & 33).
- Aggregates individual student signals into class-wide misconception insights.
- Cohort Intelligence Dashboard: top misconceptions, question clusters, and lecture resonance (zero PII).
- Targeted Pedagogical Intervention: dispatches high-priority revision packs into struggling students' Today timelines.
"""
import uuid
import logging
from datetime import datetime, timezone
from typing import List, Dict, Any, Optional
from sqlalchemy.orm import Session
from sqlalchemy import func

from academic_service.models.orm import (
    Course,
    Lecture,
    StudentKnowledgeState,
    StudentEnrollment,
    LearningGap,
    LecturerClassSignal,
    ProactiveIntervention,
)
from academic_service.models.schema import LecturerClassSignalDTO

logger = logging.getLogger(__name__)


class LecturerSignalService:
    """
    Lecturer intelligence dashboard and pedagogical intervention dispatcher.
    """

    # =========================================================================
    # Phase 3: Class Signal Aggregation
    # =========================================================================

    @staticmethod
    def get_course_class_signals(db: Session, course_id: str) -> List[LecturerClassSignalDTO]:
        """
        Aggregate class-wide student performance to identify concepts where the cohort is struggling.
        """
        course = (
            db.query(Course)
            .filter((Course.id == course_id) | (Course.code.ilike(course_id.strip())))
            .first()
        )
        if not course:
            return []

        # Count total active enrolled students
        total_students = (
            db.query(StudentEnrollment)
            .filter(StudentEnrollment.course_id == course.id, StudentEnrollment.status == "active")
            .count()
        )
        if total_students == 0:
            total_students = 1

        states = (
            db.query(StudentKnowledgeState)
            .filter(StudentKnowledgeState.course_id == course.id)
            .all()
        )

        subtopic_stats: Dict[str, Dict[str, Any]] = {}
        for s in states:
            key = f"{s.topic}::{s.subtopic}"
            if key not in subtopic_stats:
                subtopic_stats[key] = {
                    "topic": s.topic,
                    "subtopic": s.subtopic,
                    "evaluated_students": 0,
                    "struggling_count": 0,
                    "total_score": 0,
                }
            subtopic_stats[key]["evaluated_students"] += 1
            subtopic_stats[key]["total_score"] += s.mastery_score
            if s.status == "struggling" or s.mastery_score < 50:
                subtopic_stats[key]["struggling_count"] += 1

        signals = []
        for key, data in subtopic_stats.items():
            evaluated_count = max(1, data["evaluated_students"])
            struggling_pct = int((data["struggling_count"] / evaluated_count) * 100)

            recommendation = None
            if struggling_pct >= 50:
                recommendation = (
                    f"{struggling_pct}% of students struggled with {data['subtopic']} on recent diagnostics. "
                    f"Recommended action: Dedicate 10-15 minutes in the next lecture to review core definitions and state transitions."
                )
            elif struggling_pct >= 30:
                recommendation = f"{struggling_pct}% have partial understanding of {data['subtopic']}. Consider sharing an extra worked example."

            signals.append(
                LecturerClassSignalDTO(
                    course_id=course.id,
                    course_code=course.code,
                    topic=data["topic"],
                    subtopic=data["subtopic"],
                    total_students=evaluated_count,
                    struggling_count=data["struggling_count"],
                    struggling_percentage=struggling_pct,
                    recommendation=recommendation,
                )
            )

        signals.sort(key=lambda s: s.struggling_percentage, reverse=True)
        return signals

    # =========================================================================
    # Phase 32: Cohort Intelligence Dashboard (Zero Student PII)
    # =========================================================================

    @classmethod
    def get_cohort_intelligence_dashboard(
        cls,
        db: Session,
        course_id: str,
    ) -> Dict[str, Any]:
        """
        Returns anonymized cohort analytics for the lecturer:
        - Top 3 prevalent misconceptions across the cohort.
        - Question frequency clusters.
        - Lecture resonance metrics.
        Zero student PII is exposed.
        """
        course = (
            db.query(Course)
            .filter((Course.id == course_id) | (Course.code.ilike(course_id.strip())))
            .first()
        )
        if not course:
            raise ValueError(f"Course '{course_id}' not found.")

        total_enrolled = (
            db.query(StudentEnrollment)
            .filter(StudentEnrollment.course_id == course.id, StudentEnrollment.status == "active")
            .count()
        )
        cohort_size = max(1, total_enrolled)

        # 1. Anonymized Top Misconceptions (Aggregated from LearningGap records)
        active_gaps = (
            db.query(LearningGap)
            .filter(LearningGap.course_id == course.id, LearningGap.status == "active")
            .all()
        )

        misconception_freq: Dict[str, Dict[str, Any]] = {}
        for g in active_gaps:
            desc = g.gap_description.strip()
            # Normalize description
            key = f"{g.subtopic}::{desc}"
            if key not in misconception_freq:
                misconception_freq[key] = {
                    "topic": g.topic,
                    "subtopic": g.subtopic,
                    "misconception": desc,
                    "affected_students_count": 0,
                    "severity": g.severity,
                }
            misconception_freq[key]["affected_students_count"] += 1

        top_misconceptions_list = sorted(
            misconception_freq.values(),
            key=lambda m: m["affected_students_count"],
            reverse=True,
        )[:3]

        for m in top_misconceptions_list:
            m["percentage_of_cohort"] = int((m["affected_students_count"] / cohort_size) * 100)

        # 2. Question Frequency Clusters
        class_signals = cls.get_course_class_signals(db, course.id)
        question_clusters = []
        for cs in class_signals[:4]:
            question_clusters.append({
                "cluster_topic": cs.topic,
                "subtopic": cs.subtopic,
                "struggling_percentage": cs.struggling_percentage,
                "struggling_students_count": cs.struggling_count,
                "inquiry_heat": "high" if cs.struggling_percentage >= 50 else ("medium" if cs.struggling_percentage >= 25 else "low"),
            })

        # 3. Lecture Resonance Metrics
        lectures = (
            db.query(Lecture)
            .filter(Lecture.course_id == course.id)
            .order_by(Lecture.lecture_number.asc())
            .all()
        )

        lecture_resonance = []
        for lec in lectures:
            # Match topics covered to class signals
            topics = lec.topics_covered or [lec.title]
            matching_signals = [cs for cs in class_signals if cs.subtopic in topics or cs.topic in topics]
            if matching_signals:
                avg_struggle = sum(cs.struggling_percentage for cs in matching_signals) / len(matching_signals)
                resonance_score = max(10, int(100 - avg_struggle))
            else:
                resonance_score = 80  # Default baseline resonance

            resonance_status = "strong" if resonance_score >= 70 else ("moderate" if resonance_score >= 50 else "needs_review")
            rec = None
            if resonance_score < 60:
                rec = f"Review core state transitions from Lecture {lec.lecture_number} in next session."

            lecture_resonance.append({
                "lecture_id": lec.id,
                "lecture_number": lec.lecture_number,
                "title": lec.title,
                "resonance_score": resonance_score,
                "status": resonance_status,
                "recommendation": rec,
            })

        # Calculate overall cohort health
        all_states = (
            db.query(StudentKnowledgeState)
            .filter(StudentKnowledgeState.course_id == course.id)
            .all()
        )
        avg_mastery = int(sum(s.mastery_score for s in all_states) / len(all_states)) if all_states else 70
        struggling_total = sum(1 for s in all_states if s.status == "struggling" or s.mastery_score < 50)

        return {
            "course_id": course.id,
            "course_code": course.code,
            "course_title": course.title,
            "total_enrolled": total_enrolled,
            "cohort_health_index": avg_mastery,
            "total_struggling_states": struggling_total,
            "top_misconceptions": top_misconceptions_list,
            "question_clusters": question_clusters,
            "lecture_resonance": lecture_resonance,
            "generated_at": datetime.now(timezone.utc).isoformat(),
        }

    # =========================================================================
    # Phase 33: Pedagogical Intervention Dispatcher
    # =========================================================================

    @classmethod
    def dispatch_pedagogical_intervention(
        cls,
        db: Session,
        course_id: str,
        lecturer_id: str,
        topic: str,
        title: str,
        revision_pack: Dict[str, Any],
    ) -> Dict[str, Any]:
        """
        Identifies all enrolled students struggling with 'topic', and dispatches
        a high-priority pedagogical intervention card into their personalized Today timeline.
        """
        course = (
            db.query(Course)
            .filter((Course.id == course_id) | (Course.code.ilike(course_id.strip())))
            .first()
        )
        if not course:
            raise ValueError(f"Course '{course_id}' not found.")

        # Find struggling students in this topic
        struggling_user_ids = set()

        # From KnowledgeState
        ks_records = (
            db.query(StudentKnowledgeState)
            .filter(
                StudentKnowledgeState.course_id == course.id,
                (StudentKnowledgeState.topic == topic) | (StudentKnowledgeState.subtopic == topic),
                (StudentKnowledgeState.status == "struggling") | (StudentKnowledgeState.mastery_score < 50),
            )
            .all()
        )
        for ks in ks_records:
            struggling_user_ids.add(ks.user_id)

        # From LearningGap
        gaps = (
            db.query(LearningGap)
            .filter(
                LearningGap.course_id == course.id,
                (LearningGap.topic == topic) | (LearningGap.subtopic == topic),
                LearningGap.status == "active",
            )
            .all()
        )
        for g in gaps:
            struggling_user_ids.add(g.user_id)

        # Fallback: if no specific struggling records exist, target all actively enrolled students
        if not struggling_user_ids:
            enrollments = (
                db.query(StudentEnrollment)
                .filter(StudentEnrollment.course_id == course.id, StudentEnrollment.status == "active")
                .all()
            )
            for e in enrollments:
                struggling_user_ids.add(e.user_id)

        dispatched_interventions = []
        now = datetime.now(timezone.utc)

        for uid in struggling_user_ids:
            intervention = ProactiveIntervention(
                id=f"interv_{uuid.uuid4()}",
                user_id=uid,
                course_id=course.id,
                event_type="PEDAGOGICAL_INTERVENTION",
                card_type="lecturer_revision_pack",
                title=title,
                payload={
                    "lecturer_id": lecturer_id,
                    "course_code": course.code,
                    "topic": topic,
                    "priority": 1,
                    "revision_pack": revision_pack,
                    "message": f"Your lecturer shared targeted revision materials for {topic}.",
                    "dispatched_at": now.isoformat(),
                },
                is_dismissed=False,
                is_acted_upon=False,
                delivered_at=now,
            )
            db.add(intervention)
            dispatched_interventions.append(intervention)

        db.commit()

        logger.info(
            f"📢 Dispatched pedagogical intervention '{title}' to {len(struggling_user_ids)} students "
            f"in {course.code} for topic '{topic}'"
        )

        return {
            "course_id": course.id,
            "course_code": course.code,
            "lecturer_id": lecturer_id,
            "topic": topic,
            "title": title,
            "targeted_students_count": len(struggling_user_ids),
            "card_type": "lecturer_revision_pack",
            "priority": 1,
            "dispatched_at": now.isoformat(),
            "status": "dispatched",
        }

"""
Proactive Engine & "Today" Timeline Generator for Lexi (System ➔ Student).
Continuously monitors student timetables, upcoming lectures, newly processed materials,
deadlines, and learning gaps to generate dynamic timelines and spam-governed proactive interventions.
"""
import uuid
import logging
from datetime import datetime, timezone, timedelta, time, date
from typing import List, Optional, Dict, Any, Tuple
from sqlalchemy.orm import Session

from academic_service.models.orm import (
    Course,
    CourseSchedule,
    StudentEnrollment,
    Lecture,
    AcademicEvent,
    LearningGap,
    StudentKnowledgeState,
    ProactiveIntervention,
)
from academic_service.models.schema import (
    TimelineCardDTO,
    TodayTimelineResponse,
    InterventionDecisionDTO,
    WeakTopicDTO,
)
from academic_service.services.timetable_service import TimetableService
from academic_service.services.knowledge_state_service import KnowledgeStateService

logger = logging.getLogger(__name__)


class ProactiveGovernor:
    """
    Proactive Intervention Governor (Anti-Spam Filter).
    Enforces rules to prevent notification fatigue:
    - Max 2 high-priority push interventions per day (24-hour rolling window).
    - Minimum 4 hours cool-off window between unsolicited push alerts.
    - Evaluates historical engagement (suppresses alerts if student dismissed 3+ consecutive).
    - Urgency and confidence scoring.
    """
    MAX_DAILY_HIGH_PRIORITY = 2
    COOL_OFF_HOURS = 4
    FATIGUE_DISMISSAL_THRESHOLD = 3

    @classmethod
    def evaluate(
        cls,
        db: Session,
        user_id: str,
        card: TimelineCardDTO,
        event_type: str,
        current_time: Optional[datetime] = None,
        force: bool = False,
    ) -> Tuple[bool, str]:
        """
        Evaluate whether an intervention should be delivered via proactive push.
        Returns (should_intervene: bool, reason: str).
        """
        if force:
            return True, "Forced intervention dispatch."

        now = current_time or datetime.now(timezone.utc)
        if now.tzinfo is None:
            now = now.replace(tzinfo=timezone.utc)

        day_ago = now - timedelta(hours=24)
        cool_off_limit = now - timedelta(hours=cls.COOL_OFF_HOURS)

        # 1. Check historical engagement / fatigue
        recent_interventions = (
            db.query(ProactiveIntervention)
            .filter(ProactiveIntervention.user_id == user_id)
            .order_by(ProactiveIntervention.delivered_at.desc())
            .limit(5)
            .all()
        )

        if len(recent_interventions) >= cls.FATIGUE_DISMISSAL_THRESHOLD:
            # Check if the last N were dismissed and none acted upon
            last_n = recent_interventions[:cls.FATIGUE_DISMISSAL_THRESHOLD]
            if all(i.is_dismissed and not i.is_acted_upon for i in last_n):
                if card.priority > 1:
                    return False, f"Fatigue suppression: Student dismissed last {cls.FATIGUE_DISMISSAL_THRESHOLD} interventions without action. Non-critical alert blocked."

        # 2. Check Daily High-Priority Cap (Max 2 per day)
        if card.priority == 1:
            high_priority_today_count = (
                db.query(ProactiveIntervention)
                .filter(
                    ProactiveIntervention.user_id == user_id,
                    ProactiveIntervention.delivered_at >= day_ago,
                    ProactiveIntervention.event_type.in_(["CLASS_APPROACHING", "GAP_DETECTED", "EXAM_APPROACHING", "DEADLINE_APPROACHING"]),
                )
                .count()
            )
            if high_priority_today_count >= cls.MAX_DAILY_HIGH_PRIORITY:
                return False, f"Daily limit reached: Maximum {cls.MAX_DAILY_HIGH_PRIORITY} high-priority proactive interventions per 24 hours."

        # 3. Check Cool-Off Window (Min 4 hours between push alerts)
        last_delivered = (
            db.query(ProactiveIntervention)
            .filter(
                ProactiveIntervention.user_id == user_id,
                ProactiveIntervention.delivered_at >= cool_off_limit,
            )
            .order_by(ProactiveIntervention.delivered_at.desc())
            .first()
        )
        if last_delivered:
            # Exception: class starting in <= 15 minutes is critical
            is_critical_countdown = False
            if card.card_type in ["class_countdown", "pre_class_prep"]:
                if card.action_payload and card.action_payload.get("minutes_until_class", 999) <= 15:
                    is_critical_countdown = True

            if not is_critical_countdown:
                last_time = last_delivered.delivered_at
                if last_time.tzinfo is None:
                    last_time = last_time.replace(tzinfo=timezone.utc)
                elapsed_mins = max(0, int((now - last_time).total_seconds() // 60))
                return False, f"Cool-off window active: Last intervention was delivered {elapsed_mins} minutes ago (minimum cooldown is {cls.COOL_OFF_HOURS * 60} minutes)."

        return True, "Intervention passed governor checks."


class ProactiveEngine:
    """
    Main Proactive Engine generating the 'Today' timeline feed,
    evaluating trigger rules, and dispatching governed interventions.
    """

    @staticmethod
    def generate_today_timeline(
        db: Session,
        user_id: str,
        current_time: Optional[datetime] = None,
    ) -> TodayTimelineResponse:
        """
        Build the rich, dynamic 'Today' timeline for a student.
        Combines:
        1. Class Countdown / Pre-Class Prep Card (if upcoming within 60 mins & weak topic exists)
        2. Post-Class Follow-up Card (if a class recently finished)
        3. Active Learning Gap Diagnostic Card (if active learning gaps exist)
        4. Academic Deadline Card (if assignments/exams due within 48 hours)
        """
        now = current_time or datetime.now()
        current_date_str = now.strftime("%Y-%m-%d")
        current_weekday = now.isoweekday()  # 1=Mon, 7=Sun
        current_time_val = now.time()

        cards: List[TimelineCardDTO] = []

        # Get active enrolled courses
        enrollments = (
            db.query(StudentEnrollment)
            .filter(StudentEnrollment.user_id == user_id, StudentEnrollment.status == "active")
            .all()
        )
        course_ids = [e.course_id for e in enrollments]
        if not course_ids:
            return TodayTimelineResponse(
                date=current_date_str,
                current_academic_session="2025/2026_FIRST",
                cards=[],
            )

        courses_map = {
            c.id: c for c in db.query(Course).filter(Course.id.in_(course_ids)).all()
        }

        # Check existing dismissed interventions for today to avoid re-showing dismissed cards
        dismissed_records = (
            db.query(ProactiveIntervention)
            .filter(
                ProactiveIntervention.user_id == user_id,
                ProactiveIntervention.is_dismissed == True,
            )
            .all()
        )
        dismissed_ids = {r.id for r in dismissed_records}

        # =========================================================================
        # 1. UPCOMING CLASS & PRE-CLASS PREPARATION
        # =========================================================================
        next_class = TimetableService.get_next_class_for_student(db, user_id, current_time=now)
        if next_class.has_upcoming and next_class.minutes_until_class is not None:
            # Find the course matching course_code
            matching_course = None
            for c in courses_map.values():
                if c.code.strip().upper() == (next_class.course_code or "").strip().upper():
                    matching_course = c
                    break

            mins = next_class.minutes_until_class
            is_imminent = mins <= 60

            if matching_course and is_imminent:
                # Check for weak topics in this course
                weak_topics = KnowledgeStateService.get_weakest_topics(db, user_id, matching_course.id, limit=1)
                if weak_topics:
                    weak_topic = weak_topics[0]
                    card_id = f"card_preclass_{matching_course.id}_{now.strftime('%Y%m%d')}"
                    if card_id not in dismissed_ids:
                        cards.append(
                            TimelineCardDTO(
                                id=card_id,
                                card_type="pre_class_prep",
                                priority=1,
                                title=f"Pre-Class Prep: {matching_course.code}",
                                subtitle=f"Review weak topic '{weak_topic.subtopic}' before class in {mins} mins ({next_class.venue})",
                                action_type="start_diagnostic",
                                action_payload={
                                    "course_id": matching_course.id,
                                    "course_code": matching_course.code,
                                    "topic": weak_topic.topic,
                                    "subtopic": weak_topic.subtopic,
                                    "mastery_score": weak_topic.mastery_score,
                                    "venue": next_class.venue,
                                    "minutes_until_class": mins,
                                    "start_time": next_class.start_time,
                                },
                                estimated_minutes=4,
                                course_id=matching_course.id,
                                course_code=matching_course.code,
                                created_at=now.isoformat(),
                            )
                        )
                else:
                    # Imminent class, no weak topic
                    card_id = f"card_countdown_{matching_course.id}_{now.strftime('%Y%m%d')}"
                    if card_id not in dismissed_ids:
                        cards.append(
                            TimelineCardDTO(
                                id=card_id,
                                card_type="class_countdown",
                                priority=1 if mins <= 30 else 2,
                                title=f"Upcoming Lecture: {matching_course.code}",
                                subtitle=f"{matching_course.title} starts in {mins} mins at {next_class.venue}",
                                action_type="view_class",
                                action_payload={
                                    "course_id": matching_course.id,
                                    "course_code": matching_course.code,
                                    "venue": next_class.venue,
                                    "minutes_until_class": mins,
                                    "start_time": next_class.start_time,
                                },
                                estimated_minutes=None,
                                course_id=matching_course.id,
                                course_code=matching_course.code,
                                created_at=now.isoformat(),
                            )
                        )
            elif matching_course:
                # Class upcoming later today or this week
                card_id = f"card_countdown_{matching_course.id}_{now.strftime('%Y%m%d')}"
                if card_id not in dismissed_ids:
                    cards.append(
                        TimelineCardDTO(
                            id=card_id,
                            card_type="class_countdown",
                            priority=3,
                            title=f"Next Class: {matching_course.code}",
                            subtitle=f"{matching_course.title} starts at {next_class.start_time} ({next_class.venue})",
                            action_type="view_class",
                            action_payload={
                                "course_id": matching_course.id,
                                "course_code": matching_course.code,
                                "venue": next_class.venue,
                                "minutes_until_class": mins,
                                "start_time": next_class.start_time,
                            },
                            estimated_minutes=None,
                            course_id=matching_course.id,
                            course_code=matching_course.code,
                            created_at=now.isoformat(),
                        )
                    )

        # =========================================================================
        # 2. POST-CLASS FOLLOW-UP CARDS
        # =========================================================================
        # Check if any enrolled course had a class slot earlier today that finished
        today_schedules = (
            db.query(CourseSchedule)
            .filter(
                CourseSchedule.course_id.in_(course_ids),
                CourseSchedule.day_of_week == current_weekday,
                CourseSchedule.end_time <= current_time_val,
            )
            .all()
        )
        for sched in today_schedules:
            crs = courses_map.get(sched.course_id)
            if not crs:
                continue

            # Look for recent processed lecture or summary
            recent_lecture = (
                db.query(Lecture)
                .filter(Lecture.course_id == crs.id, Lecture.is_processed == True)
                .order_by(Lecture.lecture_number.desc())
                .first()
            )

            card_id = f"card_postclass_{crs.id}_{now.strftime('%Y%m%d')}"
            if card_id not in dismissed_ids:
                topics_str = ", ".join(recent_lecture.topics_covered) if recent_lecture and recent_lecture.topics_covered else "today's concepts"
                summary_snippet = recent_lecture.summary_text if recent_lecture and recent_lecture.summary_text else f"Review key takeaways and lecture notes for {crs.code}."

                cards.append(
                    TimelineCardDTO(
                        id=card_id,
                        card_type="post_class_summary",
                        priority=2,
                        title=f"Post-Class Takeaways: {crs.code}",
                        subtitle=f"Class ended. Review key concepts: {topics_str}",
                        action_type="review_summary",
                        action_payload={
                            "course_id": crs.id,
                            "course_code": crs.code,
                            "lecture_id": recent_lecture.id if recent_lecture else None,
                            "lecture_title": recent_lecture.title if recent_lecture else f"{crs.code} Lecture",
                            "topics_covered": recent_lecture.topics_covered if recent_lecture else [],
                            "summary": summary_snippet,
                        },
                        estimated_minutes=5,
                        course_id=crs.id,
                        course_code=crs.code,
                        created_at=now.isoformat(),
                    )
                )

        # =========================================================================
        # 3. ACTIVE LEARNING GAP CARDS
        # =========================================================================
        active_gaps = (
            db.query(LearningGap)
            .filter(
                LearningGap.user_id == user_id,
                LearningGap.course_id.in_(course_ids),
                LearningGap.status == "active",
            )
            .order_by(LearningGap.created_at.desc())
            .limit(2)
            .all()
        )
        for gap in active_gaps:
            crs = courses_map.get(gap.course_id)
            code_label = crs.code if crs else "Course"
            card_id = f"card_gap_{gap.id}"
            if card_id not in dismissed_ids:
                cards.append(
                    TimelineCardDTO(
                        id=card_id,
                        card_type="practice_gap",
                        priority=1 if gap.severity == "high" else 2,
                        title=f"Targeted Practice: {code_label}",
                        subtitle=f"3-question diagnostic to resolve: {gap.gap_description}",
                        action_type="solve_gap",
                        action_payload={
                            "gap_id": gap.id,
                            "course_id": gap.course_id,
                            "course_code": code_label,
                            "topic": gap.topic,
                            "subtopic": gap.subtopic,
                            "gap_description": gap.gap_description,
                            "severity": gap.severity,
                            "question_count": 3,
                        },
                        estimated_minutes=3,
                        course_id=gap.course_id,
                        course_code=code_label,
                        created_at=now.isoformat(),
                    )
                )

        # =========================================================================
        # 4. ACADEMIC DEADLINES (Within 48 hours)
        # =========================================================================
        horizon_48h = now + timedelta(hours=48)
        approaching_events = (
            db.query(AcademicEvent)
            .filter(
                AcademicEvent.course_id.in_(course_ids),
                AcademicEvent.due_date >= now,
                AcademicEvent.due_date <= horizon_48h,
            )
            .order_by(AcademicEvent.due_date.asc())
            .all()
        )
        for event in approaching_events:
            crs = courses_map.get(event.course_id)
            code_label = crs.code if crs else "Course"
            card_id = f"card_event_{event.id}"
            if card_id not in dismissed_ids:
                # Handle tzinfo normalization
                ev_due = event.due_date
                now_comp = now
                if ev_due.tzinfo is not None and now_comp.tzinfo is None:
                    now_comp = now_comp.replace(tzinfo=timezone.utc)
                elif ev_due.tzinfo is None and now_comp.tzinfo is not None:
                    ev_due = ev_due.replace(tzinfo=timezone.utc)

                hours_left = max(1, int((ev_due - now_comp).total_seconds() // 3600))
                cards.append(
                    TimelineCardDTO(
                        id=card_id,
                        card_type="deadline",
                        priority=1 if hours_left <= 24 else 2,
                        title=f"Deadline Approaching: {event.title}",
                        subtitle=f"Due in {hours_left}h ({event.event_type.capitalize()}) • {code_label}",
                        action_type="open_deadline",
                        action_payload={
                            "event_id": event.id,
                            "course_id": event.course_id,
                            "course_code": code_label,
                            "title": event.title,
                            "due_date": event.due_date.isoformat(),
                            "hours_remaining": hours_left,
                            "weight_percent": event.weight_percent,
                        },
                        estimated_minutes=15,
                        course_id=event.course_id,
                        course_code=code_label,
                        created_at=now.isoformat(),
                    )
                )

        # Sort cards: Priority 1 first, then Priority 2, then Priority 3
        cards.sort(key=lambda c: c.priority)

        return TodayTimelineResponse(
            date=current_date_str,
            current_academic_session="2025/2026_FIRST",
            cards=cards,
        )

    @staticmethod
    def dispatch_event(
        db: Session,
        user_id: str,
        event_type: str,
        event_payload: Dict[str, Any],
        current_time: Optional[datetime] = None,
        force: bool = False,
    ) -> InterventionDecisionDTO:
        """
        Handle incoming or scheduled academic triggers:
        - CLASS_APPROACHING
        - CLASS_ENDED
        - EXAM_APPROACHING / DEADLINE_APPROACHING
        - GAP_DETECTED
        Filters through Governor and logs intervention in DB if approved.
        """
        now = current_time or datetime.now(timezone.utc)
        course_id = event_payload.get("course_id", "")
        course = None
        if course_id:
            course = db.query(Course).filter((Course.id == course_id) | (Course.code.ilike(course_id.strip()))).first()

        card: Optional[TimelineCardDTO] = None

        if event_type == "CLASS_APPROACHING":
            mins = event_payload.get("minutes_until_class", 30)
            venue = event_payload.get("venue", "Lecture Theatre")
            code_label = course.code if course else event_payload.get("course_code", "Course")
            cid = course.id if course else course_id

            # Check weak topics
            weak_topics = KnowledgeStateService.get_weakest_topics(db, user_id, cid, limit=1) if cid else []
            if weak_topics and mins <= 60:
                wt = weak_topics[0]
                card = TimelineCardDTO(
                    id=f"int_{uuid.uuid4()}",
                    card_type="pre_class_prep",
                    priority=1,
                    title=f"Pre-Class Prep: {code_label}",
                    subtitle=f"Review weak topic '{wt.subtopic}' before class starts in {mins} mins",
                    action_type="start_diagnostic",
                    action_payload={
                        "course_id": cid,
                        "course_code": code_label,
                        "topic": wt.topic,
                        "subtopic": wt.subtopic,
                        "venue": venue,
                        "minutes_until_class": mins,
                    },
                    estimated_minutes=4,
                    course_id=cid,
                    course_code=code_label,
                    created_at=now.isoformat(),
                )
            else:
                card = TimelineCardDTO(
                    id=f"int_{uuid.uuid4()}",
                    card_type="class_countdown",
                    priority=2 if mins > 30 else 1,
                    title=f"Upcoming Lecture: {code_label}",
                    subtitle=f"Class starts in {mins} mins at {venue}",
                    action_type="view_class",
                    action_payload={
                        "course_id": cid,
                        "course_code": code_label,
                        "venue": venue,
                        "minutes_until_class": mins,
                    },
                    estimated_minutes=None,
                    course_id=cid,
                    course_code=code_label,
                    created_at=now.isoformat(),
                )

        elif event_type == "CLASS_ENDED":
            code_label = course.code if course else event_payload.get("course_code", "Course")
            cid = course.id if course else course_id
            topics = event_payload.get("topics_covered", [])
            topics_str = ", ".join(topics) if topics else "today's concepts"

            card = TimelineCardDTO(
                id=f"int_{uuid.uuid4()}",
                card_type="post_class_summary",
                priority=2,
                title=f"Post-Class Takeaways: {code_label}",
                subtitle=f"Class ended. Review key takeaways: {topics_str}",
                action_type="review_summary",
                action_payload={
                    "course_id": cid,
                    "course_code": code_label,
                    "topics_covered": topics,
                    "summary": event_payload.get("summary_text", f"Review summary notes for {code_label}"),
                },
                estimated_minutes=5,
                course_id=cid,
                course_code=code_label,
                created_at=now.isoformat(),
            )

        elif event_type in ["EXAM_APPROACHING", "DEADLINE_APPROACHING"]:
            code_label = course.code if course else event_payload.get("course_code", "Course")
            cid = course.id if course else course_id
            title = event_payload.get("title", f"{code_label} Deadline")
            hours_remaining = event_payload.get("hours_remaining", 24)

            card = TimelineCardDTO(
                id=f"int_{uuid.uuid4()}",
                card_type="deadline",
                priority=1 if hours_remaining <= 24 else 2,
                title=f"Upcoming Deadline: {title}",
                subtitle=f"Due in {hours_remaining} hours • {code_label}",
                action_type="open_deadline",
                action_payload={
                    "course_id": cid,
                    "course_code": code_label,
                    "title": title,
                    "hours_remaining": hours_remaining,
                    "weight_percent": event_payload.get("weight_percent"),
                },
                estimated_minutes=15,
                course_id=cid,
                course_code=code_label,
                created_at=now.isoformat(),
            )

        elif event_type == "GAP_DETECTED":
            code_label = course.code if course else event_payload.get("course_code", "Course")
            cid = course.id if course else course_id
            topic = event_payload.get("topic", "Core Topic")
            subtopic = event_payload.get("subtopic", "Concept")
            gap_desc = event_payload.get("gap_description", "Struggling with conceptual understanding")
            severity = event_payload.get("severity", "medium")

            card = TimelineCardDTO(
                id=f"int_{uuid.uuid4()}",
                card_type="practice_gap",
                priority=1 if severity == "high" else 2,
                title=f"Targeted Practice: {code_label}",
                subtitle=f"3-question diagnostic to resolve: {gap_desc}",
                action_type="solve_gap",
                action_payload={
                    "course_id": cid,
                    "course_code": code_label,
                    "topic": topic,
                    "subtopic": subtopic,
                    "gap_description": gap_desc,
                    "severity": severity,
                    "question_count": 3,
                },
                estimated_minutes=3,
                course_id=cid,
                course_code=code_label,
                created_at=now.isoformat(),
            )

        else:
            return InterventionDecisionDTO(
                should_intervene=False,
                reason=f"Unknown or unhandled event_type '{event_type}'",
                card=None,
            )

        # Run Governor check
        can_deliver, reason = ProactiveGovernor.evaluate(
            db=db,
            user_id=user_id,
            card=card,
            event_type=event_type,
            current_time=now,
            force=force,
        )

        if can_deliver:
            # Persist intervention record
            intervention_record = ProactiveIntervention(
                id=card.id,
                user_id=user_id,
                course_id=card.course_id,
                event_type=event_type,
                card_type=card.card_type,
                title=card.title,
                payload=card.action_payload,
                is_dismissed=False,
                is_acted_upon=False,
                delivered_at=now if now.tzinfo else now.replace(tzinfo=timezone.utc),
            )
            db.add(intervention_record)
            db.commit()

            return InterventionDecisionDTO(
                should_intervene=True,
                reason=reason,
                card=card,
            )
        else:
            return InterventionDecisionDTO(
                should_intervene=False,
                reason=reason,
                card=card,
            )

    @staticmethod
    def act_on_intervention(db: Session, user_id: str, intervention_id: str) -> Dict[str, Any]:
        """Record that student clicked or engaged with a timeline card / intervention."""
        record = (
            db.query(ProactiveIntervention)
            .filter(ProactiveIntervention.id == intervention_id, ProactiveIntervention.user_id == user_id)
            .first()
        )
        if not record:
            # If not already persisted as an explicit intervention, create an acted record
            record = ProactiveIntervention(
                id=intervention_id,
                user_id=user_id,
                event_type="TIMELINE_ACTED",
                card_type="timeline_card",
                title=f"Acted card {intervention_id}",
                is_dismissed=False,
                is_acted_upon=True,
                delivered_at=datetime.now(timezone.utc),
            )
            db.add(record)
        else:
            record.is_acted_upon = True

        db.commit()
        return {
            "status": "success",
            "message": "Student action recorded successfully",
            "intervention_id": intervention_id,
            "learning_signal_recorded": True,
        }

    @staticmethod
    def dismiss_intervention(db: Session, user_id: str, intervention_id: str) -> Dict[str, Any]:
        """Record dismissal of a timeline card / intervention to avoid re-prompting."""
        record = (
            db.query(ProactiveIntervention)
            .filter(ProactiveIntervention.id == intervention_id, ProactiveIntervention.user_id == user_id)
            .first()
        )
        if not record:
            record = ProactiveIntervention(
                id=intervention_id,
                user_id=user_id,
                event_type="TIMELINE_DISMISSED",
                card_type="timeline_card",
                title=f"Dismissed card {intervention_id}",
                is_dismissed=True,
                is_acted_upon=False,
                delivered_at=datetime.now(timezone.utc),
            )
            db.add(record)
        else:
            record.is_dismissed = True

        db.commit()
        return {
            "status": "success",
            "message": "Card dismissed successfully",
            "intervention_id": intervention_id,
        }

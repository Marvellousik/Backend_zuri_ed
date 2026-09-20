"""
Unit tests for Proactive Engine, Today Timeline Generation, and Proactive Governor (Anti-Spam Filter).
"""
from datetime import datetime, time, timedelta, timezone
from unittest.mock import MagicMock, patch
from sqlalchemy.orm import Session

from academic_service.services.proactive_engine import ProactiveEngine, ProactiveGovernor
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
    NextClassResponse,
    WeakTopicDTO,
)


def test_timeline_generation_pre_class_prep():
    """Verify that when a class is approaching within 60 minutes and student has a weak topic, a pre-class prep card is generated."""
    db_mock = MagicMock(spec=Session)

    # 1. Mock enrollment
    enrollment = StudentEnrollment(user_id="usr_demo", course_id="course_csc301", status="active")
    # 2. Mock course
    course = Course(
        id="course_csc301",
        code="CSC 301",
        title="Data Structures & Algorithms",
    )

    def query_side_effect(model):
        m = MagicMock()
        if model == StudentEnrollment:
            m.filter.return_value.all.return_value = [enrollment]
        elif model == Course:
            m.filter.return_value.all.return_value = [course]
            m.filter.return_value.first.return_value = course
        elif model == ProactiveIntervention:
            m.filter.return_value.all.return_value = []
        elif model == CourseSchedule:
            m.filter.return_value.all.return_value = []
        elif model == LearningGap:
            m.filter.return_value.order_by.return_value.limit.return_value.all.return_value = []
        elif model == AcademicEvent:
            m.filter.return_value.order_by.return_value.all.return_value = []
        return m

    db_mock.query.side_effect = query_side_effect

    # 4. Mock timetable next class response (25 minutes until class)
    simulated_now = datetime(2026, 8, 31, 8, 35, 0) # Monday 08:35:00
    mock_next_class = NextClassResponse(
        has_upcoming=True,
        course_code="CSC 301",
        course_title="Data Structures & Algorithms",
        day_of_week=1,
        start_time="09:00:00",
        venue="Science Lab 2",
        minutes_until_class=25,
    )

    # 5. Mock weak topic in CSC 301
    weak_topic = WeakTopicDTO(
        course_id="course_csc301",
        course_code="CSC 301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        mastery_score=30,
        status="struggling",
        reason="Repeatedly misses base cases",
    )

    with patch("academic_service.services.proactive_engine.TimetableService.get_next_class_for_student", return_value=mock_next_class), \
         patch("academic_service.services.proactive_engine.KnowledgeStateService.get_weakest_topics", return_value=[weak_topic]):
        
        timeline = ProactiveEngine.generate_today_timeline(
            db=db_mock,
            user_id="usr_demo",
            current_time=simulated_now,
        )

        assert len(timeline.cards) == 1
        card = timeline.cards[0]
        assert card.card_type == "pre_class_prep"
        assert card.priority == 1
        assert "CSC 301" in card.title
        assert "Optimal Substructure" in card.subtitle
        assert card.estimated_minutes == 4
        assert card.action_type == "start_diagnostic"
        assert card.action_payload["mastery_score"] == 30


def test_timeline_generation_post_class_summary():
    """Verify that a recently completed lecture generates a post-class summary review card."""
    db_mock = MagicMock(spec=Session)

    enrollment = StudentEnrollment(user_id="usr_demo", course_id="course_csc301", status="active")
    course = Course(id="course_csc301", code="CSC 301", title="Data Structures")

    finished_sched = CourseSchedule(
        id="s1",
        course_id="course_csc301",
        day_of_week=1,
        start_time=time(9, 0),
        end_time=time(11, 0),
        venue="Science Lab 2",
    )

    recent_lecture = Lecture(
        id="lec_1",
        course_id="course_csc301",
        lecture_number=14,
        title="Optimal Substructure & Recurrences",
        topics_covered=["Optimal Substructure", "Base Cases"],
        summary_text="Explored state transitions and optimal subproblems.",
        is_processed=True,
    )

    def query_side_effect(model):
        m = MagicMock()
        if model == StudentEnrollment:
            m.filter.return_value.all.return_value = [enrollment]
        elif model == Course:
            m.filter.return_value.all.return_value = [course]
        elif model == ProactiveIntervention:
            m.filter.return_value.all.return_value = []
        elif model == CourseSchedule:
            m.filter.return_value.all.return_value = [finished_sched]
        elif model == Lecture:
            m.filter.return_value.order_by.return_value.first.return_value = recent_lecture
        elif model == LearningGap:
            m.filter.return_value.order_by.return_value.limit.return_value.all.return_value = []
        elif model == AcademicEvent:
            m.filter.return_value.order_by.return_value.all.return_value = []
        return m

    db_mock.query.side_effect = query_side_effect

    mock_next_class = NextClassResponse(has_upcoming=False)
    simulated_now = datetime(2026, 8, 31, 11, 30, 0)

    with patch("academic_service.services.proactive_engine.TimetableService.get_next_class_for_student", return_value=mock_next_class):
        timeline = ProactiveEngine.generate_today_timeline(
            db=db_mock,
            user_id="usr_demo",
            current_time=simulated_now,
        )

        assert len(timeline.cards) == 1
        card = timeline.cards[0]
        assert card.card_type == "post_class_summary"
        assert card.priority == 2
        assert "CSC 301" in card.title
        assert card.action_type == "review_summary"
        assert "Optimal Substructure" in card.subtitle


def test_timeline_generation_active_gap_and_deadline():
    """Verify that active learning gaps and approaching deadlines (<48h) populate high-priority cards."""
    db_mock = MagicMock(spec=Session)

    enrollment = StudentEnrollment(user_id="usr_demo", course_id="course_csc301", status="active")
    course = Course(id="course_csc301", code="CSC 301", title="Data Structures")

    simulated_now = datetime(2026, 8, 31, 12, 0, 0)

    gap = LearningGap(
        id="gap_1",
        user_id="usr_demo",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Base Cases",
        gap_description="Repeatedly misses base cases in recurrence formulations",
        severity="high",
        status="active",
        created_at=datetime(2026, 8, 30, 10, 0, 0),
    )

    event = AcademicEvent(
        id="ev_1",
        course_id="course_csc301",
        event_type="assignment",
        title="Lab 2 Recurrences",
        due_date=simulated_now + timedelta(hours=18),
        weight_percent=10,
    )

    def query_side_effect(model):
        m = MagicMock()
        if model == StudentEnrollment:
            m.filter.return_value.all.return_value = [enrollment]
        elif model == Course:
            m.filter.return_value.all.return_value = [course]
        elif model == ProactiveIntervention:
            m.filter.return_value.all.return_value = []
        elif model == CourseSchedule:
            m.filter.return_value.all.return_value = []
        elif model == LearningGap:
            m.filter.return_value.order_by.return_value.limit.return_value.all.return_value = [gap]
        elif model == AcademicEvent:
            m.filter.return_value.order_by.return_value.all.return_value = [event]
        return m

    db_mock.query.side_effect = query_side_effect

    mock_next_class = NextClassResponse(has_upcoming=False)

    with patch("academic_service.services.proactive_engine.TimetableService.get_next_class_for_student", return_value=mock_next_class):
        timeline = ProactiveEngine.generate_today_timeline(
            db=db_mock,
            user_id="usr_demo",
            current_time=simulated_now,
        )

        assert len(timeline.cards) == 2
        types = [c.card_type for c in timeline.cards]
        assert "practice_gap" in types
        assert "deadline" in types

        gap_card = next(c for c in timeline.cards if c.card_type == "practice_gap")
        assert gap_card.priority == 1
        assert gap_card.action_type == "solve_gap"
        assert gap_card.estimated_minutes == 3

        deadline_card = next(c for c in timeline.cards if c.card_type == "deadline")
        assert deadline_card.priority == 1
        assert deadline_card.action_payload["hours_remaining"] == 18


def test_governor_daily_high_priority_cap():
    """Verify that ProactiveGovernor limits high-priority unsolicited interventions to max 2 per day."""
    db_mock = MagicMock(spec=Session)

    now = datetime(2026, 9, 1, 14, 0, 0, tzinfo=timezone.utc)
    db_mock.query(ProactiveIntervention).filter().order_by().limit().all.return_value = [] # no fatigue

    # Mock that 2 high priority interventions already delivered in the past 24 hours
    db_mock.query(ProactiveIntervention).filter().count.return_value = 2

    card = TimelineCardDTO(
        id="card_test_1",
        card_type="practice_gap",
        priority=1,
        title="Diagnostic Alert",
        action_type="solve_gap",
    )

    should_intervene, reason = ProactiveGovernor.evaluate(
        db=db_mock,
        user_id="usr_demo",
        card=card,
        event_type="GAP_DETECTED",
        current_time=now,
    )

    assert should_intervene is False
    assert "Daily limit reached" in reason


def test_governor_cool_off_window():
    """Verify that unsolicited interventions are suppressed during the 4-hour cool-off window."""
    db_mock = MagicMock(spec=Session)

    now = datetime(2026, 9, 1, 14, 0, 0, tzinfo=timezone.utc)
    db_mock.query(ProactiveIntervention).filter().order_by().limit().all.return_value = [] # no fatigue
    db_mock.query(ProactiveIntervention).filter().count.return_value = 0 # under daily cap

    # Mock an intervention delivered 1 hour ago (13:00)
    last_intervention = ProactiveIntervention(
        id="prev_1",
        user_id="usr_demo",
        delivered_at=now - timedelta(hours=1),
        is_dismissed=False,
        is_acted_upon=True,
    )
    db_mock.query(ProactiveIntervention).filter().order_by().first.return_value = last_intervention

    card = TimelineCardDTO(
        id="card_test_2",
        card_type="post_class_summary",
        priority=2,
        title="Review Today's Notes",
        action_type="review_summary",
    )

    should_intervene, reason = ProactiveGovernor.evaluate(
        db=db_mock,
        user_id="usr_demo",
        card=card,
        event_type="CLASS_ENDED",
        current_time=now,
    )

    assert should_intervene is False
    assert "Cool-off window active" in reason


def test_governor_fatigue_suppression():
    """Verify that student dismissing 3+ consecutive interventions triggers fatigue suppression for non-critical alerts."""
    db_mock = MagicMock(spec=Session)

    now = datetime(2026, 9, 1, 14, 0, 0, tzinfo=timezone.utc)

    # 3 consecutive dismissed interventions
    dismissed_list = [
        ProactiveIntervention(id="d1", user_id="usr_demo", is_dismissed=True, is_acted_upon=False),
        ProactiveIntervention(id="d2", user_id="usr_demo", is_dismissed=True, is_acted_upon=False),
        ProactiveIntervention(id="d3", user_id="usr_demo", is_dismissed=True, is_acted_upon=False),
    ]
    db_mock.query(ProactiveIntervention).filter().order_by().limit().all.return_value = dismissed_list

    card = TimelineCardDTO(
        id="card_test_3",
        card_type="post_class_summary",
        priority=2, # Medium priority
        title="Notes ready",
        action_type="review_summary",
    )

    should_intervene, reason = ProactiveGovernor.evaluate(
        db=db_mock,
        user_id="usr_demo",
        card=card,
        event_type="CLASS_ENDED",
        current_time=now,
    )

    assert should_intervene is False
    assert "Fatigue suppression" in reason


def test_dispatch_event_and_action_tracking():
    """Verify dispatching an event generates and saves an intervention, and acting marks is_acted_upon."""
    db_mock = MagicMock(spec=Session)

    course = Course(id="course_csc301", code="CSC 301", title="Data Structures")
    db_mock.query(Course).filter().first.return_value = course

    # Governor passes
    with patch.object(ProactiveGovernor, "evaluate", return_value=(True, "Approved")):
        decision = ProactiveEngine.dispatch_event(
            db=db_mock,
            user_id="usr_demo",
            event_type="GAP_DETECTED",
            event_payload={
                "course_id": "course_csc301",
                "topic": "Dynamic Programming",
                "subtopic": "Base Cases",
                "gap_description": "Misses base cases in recurrence formulations",
                "severity": "high",
            },
        )

        assert decision.should_intervene is True
        assert decision.card is not None
        assert decision.card.card_type == "practice_gap"
        assert decision.card.action_type == "solve_gap"
        assert db_mock.add.called
        assert db_mock.commit.called

    # Test act on intervention
    intervention_record = ProactiveIntervention(
        id="int_123",
        user_id="usr_demo",
        is_dismissed=False,
        is_acted_upon=False,
    )
    db_mock.query(ProactiveIntervention).filter().first.return_value = intervention_record

    act_res = ProactiveEngine.act_on_intervention(db_mock, "usr_demo", "int_123")
    assert act_res["status"] == "success"
    assert intervention_record.is_acted_upon is True

    # Test dismiss intervention
    dismiss_res = ProactiveEngine.dismiss_intervention(db_mock, "usr_demo", "int_123")
    assert dismiss_res["status"] == "success"
    assert intervention_record.is_dismissed is True

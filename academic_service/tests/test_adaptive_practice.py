"""
Unit tests for Phase 31: Contextual Assessment & Practice Engine.
Verifies difficulty tier calibration, targeted practice session generation from active gaps,
rubric scoring, latency/confidence telemetry integration, and gap resolution.
"""
import pytest
from sqlalchemy import create_engine, event
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from academic_service.models.orm import (
    Base,
    Course,
    StudentKnowledgeState,
    LearningGap,
)
from academic_service.services.adaptive_practice_service import AdaptivePracticeService


@pytest.fixture
def db_session():
    """In-memory SQLite database session fixture with attached academic schema."""
    engine = create_engine(
        "sqlite://",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )

    @event.listens_for(engine, "connect")
    def do_connect(dbapi_connection, connection_record):
        cursor = dbapi_connection.cursor()
        cursor.execute("ATTACH DATABASE ':memory:' AS academic")
        cursor.close()

    Base.metadata.create_all(bind=engine)
    Session = sessionmaker(bind=engine)
    session = Session()
    try:
        # Seed course
        course = Course(
            id="course_csc301",
            institution_id="inst_veritas",
            department_id="dept_cs",
            code="CSC 301",
            title="Data Structures and Algorithms",
            syllabus=[
                {"topic": "Dynamic Programming", "subtopics": ["Optimal Substructure", "Memoization"]},
            ],
        )
        session.add(course)
        session.commit()
        yield session
    finally:
        session.close()


def test_difficulty_tier_calculation():
    """Verify mastery score mapping to difficulty tiers (1 to 5)."""
    assert AdaptivePracticeService.calculate_difficulty_level(15) == 1
    assert AdaptivePracticeService.calculate_difficulty_level(40) == 2
    assert AdaptivePracticeService.calculate_difficulty_level(65) == 3
    assert AdaptivePracticeService.calculate_difficulty_level(78) == 4
    assert AdaptivePracticeService.calculate_difficulty_level(95) == 5


def test_generate_adaptive_practice_session_targets_gap(db_session):
    """
    Verify generation of adaptive practice session specifically targeting an active LearningGap.
    """
    gap = LearningGap(
        id="gap_dp_01",
        user_id="student_101",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        gap_description="Repeatedly confuses optimal substructure with greedy choice.",
        severity="high",
        status="active",
    )
    ks = StudentKnowledgeState(
        id="ks_dp_01",
        user_id="student_101",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        mastery_score=25,
        status="struggling",
    )
    db_session.add_all([gap, ks])
    db_session.commit()

    session = AdaptivePracticeService.generate_adaptive_practice_session(
        db=db_session,
        user_id="student_101",
        course_id="CSC 301",
        gap_id="gap_dp_01",
    )

    assert session["session_id"].startswith("prac_")
    assert session["target_gap_id"] == "gap_dp_01"
    assert session["target_topic"] == "Dynamic Programming"
    assert session["target_subtopic"] == "Optimal Substructure"
    assert session["difficulty_level"] == 1  # 25% mastery -> Tier 1
    assert session["total_questions"] == 3
    assert session["max_points"] == 7

    # Check theory rubric presence
    theory_q = next(q for q in session["questions"] if q["type"] == "theory_rubric")
    assert "rubric" in theory_q
    assert len(theory_q["rubric"]["criteria"]) == 3
    assert theory_q["points"] == 5


def test_submit_practice_attempt_with_telemetry_and_resolution(db_session):
    """
    Verify attempt submission, telemetry processing, mastery update, and gap resolution.
    """
    gap = LearningGap(
        id="gap_dp_resolve",
        user_id="student_102",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        gap_description="Confuses overlapping subproblems.",
        severity="medium",
        status="active",
    )
    ks = StudentKnowledgeState(
        id="ks_dp_resolve",
        user_id="student_102",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        mastery_score=60,
        status="learning",
        total_attempts=2,
        correct_attempts=1,
    )
    db_session.add_all([gap, ks])
    db_session.commit()

    attempt_data = {
        "session_id": "prac_test_resolve",
        "topic": "Dynamic Programming",
        "subtopic": "Optimal Substructure",
        "gap_id": "gap_dp_resolve",
        "answers": {
            "q_1": "A",
            "q_2": "B",
            "q_3": "State representation S(i, j) with base condition S(0, j)=0 and transition max(S(i-1, j), S(i-1, j-w)+v)",
        },
        "theory_points": 5,
        "latency_ms": 4200,  # Fast correct (fluency bonus)
        "confidence_level": 5,  # Confident
    }

    result = AdaptivePracticeService.submit_practice_attempt(
        db=db_session,
        user_id="student_102",
        course_id="CSC 301",
        attempt_data=attempt_data,
    )

    assert result["points_earned"] == 7
    assert result["total_possible"] == 7
    assert result["percentage"] == 100
    assert result["updated_mastery_score"] >= 75
    assert result["gap_resolved"] is True

    # Confirm DB gap status is marked resolved
    db_gap = db_session.query(LearningGap).filter(LearningGap.id == "gap_dp_resolve").first()
    assert db_gap.status == "resolved"
    assert db_gap.resolved_at is not None


def test_submit_practice_attempt_struggling_keeps_gap_active(db_session):
    """
    Verify that an attempt with errors keeps the gap active and maintains struggling status.
    """
    gap = LearningGap(
        id="gap_dp_stay",
        user_id="student_103",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        gap_description="Repeatedly fails to identify state variables.",
        severity="medium",
        status="active",
    )
    db_session.add(gap)
    db_session.commit()

    attempt_data = {
        "session_id": "prac_test_fail",
        "topic": "Dynamic Programming",
        "subtopic": "Optimal Substructure",
        "gap_id": "gap_dp_stay",
        "answers": {
            "q_1": "C",  # Incorrect
            "q_2": "A",  # Incorrect
            "q_3": "I don't know the state representation.",
        },
        "theory_points": 0,
        "latency_ms": 26000,  # Slow incorrect
        "confidence_level": 4,  # Confidently wrong
    }

    result = AdaptivePracticeService.submit_practice_attempt(
        db=db_session,
        user_id="student_103",
        course_id="CSC 301",
        attempt_data=attempt_data,
    )

    assert result["points_earned"] == 0
    assert result["gap_resolved"] is False
    assert result["updated_mastery_score"] < 50

    db_gap = db_session.query(LearningGap).filter(LearningGap.id == "gap_dp_stay").first()
    assert db_gap.status == "active"
    assert db_gap.severity == "high"  # Escalated due to misconception trap

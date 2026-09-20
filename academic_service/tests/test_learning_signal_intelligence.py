"""
Unit tests for Phase 26: Learning Signal Intelligence.
Verifies response latency telemetry weighting, confidence level weighting,
Dunning-Kruger misconception trap detection, and dynamic LearningGap generation and resolution.
"""
from datetime import datetime, timezone
import pytest
from sqlalchemy import create_engine, event
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from academic_service.models.orm import (
    Base,
    Course,
    StudentKnowledgeState,
    LearningSignal,
    LearningGap,
)
from academic_service.models.schema import LearningSignalCreate
from academic_service.services.signal_ingestion_service import SignalIngestionService


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
        # Seed test course
        course = Course(
            id="course_csc301",
            institution_id="inst_veritas",
            department_id="dept_cs",
            code="CSC 301",
            title="Data Structures and Algorithms",
        )
        session.add(course)
        session.commit()
        yield session
    finally:
        session.close()


def test_latency_weighting_fast_correct_bonus():
    """
    Verify that fast latency (< 6s) on a correct answer applies a fluency bonus.
    """
    adj_pct, alpha, hesitation, trap = SignalIngestionService.compute_signal_weighting(
        base_score_pct=100.0,
        latency_ms=3500,  # 3.5s - Fast
        confidence_level=4,
    )

    # 100 * 1.15 * 1.10 = 126.5 -> clamped to 100
    assert adj_pct == 100
    assert hesitation is False
    assert trap is False

    # Test on an 80% base score
    adj_pct_80, _, _, _ = SignalIngestionService.compute_signal_weighting(
        base_score_pct=80.0,
        latency_ms=4000,
        confidence_level=3,
    )
    # 80 * 1.15 * 1.0 = 92
    assert adj_pct_80 == 92


def test_latency_weighting_slow_correct_tentative_mastery():
    """
    Verify that prolonged latency (> 20s) on a correct answer dampens score and flags hesitation.
    """
    adj_pct, alpha, hesitation, trap = SignalIngestionService.compute_signal_weighting(
        base_score_pct=100.0,
        latency_ms=28000,  # 28s - Hesitant / Slow
        confidence_level=3,
    )

    # 100 * 0.85 * 1.0 = 85
    assert adj_pct == 85
    assert hesitation is True
    assert trap is False


def test_confidence_weighting_lucky_guess_protection():
    """
    Verify that low confidence (1 or 2) on a correct answer dampens the update (lucky guess protection).
    """
    adj_pct, alpha, hesitation, trap = SignalIngestionService.compute_signal_weighting(
        base_score_pct=100.0,
        latency_ms=10000,  # Normal latency
        confidence_level=1,  # Guessing / Pure Uncertainty
    )

    # 100 * 1.0 * 0.80 = 80
    assert adj_pct == 80
    assert alpha == 0.20  # Reduced EMA alpha to protect historical mastery


def test_confidence_weighting_misconception_trap():
    """
    Verify that high confidence (4 or 5) on an incorrect answer triggers misconception trap flag.
    """
    adj_pct, alpha, hesitation, trap = SignalIngestionService.compute_signal_weighting(
        base_score_pct=0.0,
        latency_ms=12000,
        confidence_level=5,  # Completely certain but wrong
    )

    assert adj_pct == 0
    assert trap is True
    assert alpha == 0.50  # Heavy penalty weight


def test_dynamic_learning_gap_creation_on_low_mastery(db_session):
    """
    Verify dynamic creation of LearningGap when student scores poorly (< 50%).
    """
    signal = LearningSignalCreate(
        course_id="CSC 301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        signal_type="quiz_attempt",
        score=0,
        max_score=1,
        latency_ms=25000,  # Slow incorrect
        confidence_level=5,  # Misconception trap
        details={"error_summary": "Confused greedy choice with optimal substructure."},
    )

    dto = SignalIngestionService.record_signal(db_session, "student_01", signal)

    assert dto.mastery_score < 50
    assert dto.status == "struggling"

    # Verify LearningGap was created with 'high' severity
    gap = (
        db_session.query(LearningGap)
        .filter(
            LearningGap.user_id == "student_01",
            LearningGap.topic == "Dynamic Programming",
            LearningGap.status == "active",
        )
        .first()
    )

    assert gap is not None
    assert gap.severity == "high"
    assert "Misconception" in gap.gap_description

    # Verify LearningSignal stored telemetry
    sig_record = db_session.query(LearningSignal).filter(LearningSignal.user_id == "student_01").first()
    assert sig_record.latency_ms == 25000
    assert sig_record.confidence_level == 5


def test_dynamic_learning_gap_resolution_on_demonstrated_mastery(db_session):
    """
    Verify that an active LearningGap is automatically resolved when the student demonstrates mastery.
    """
    # 1. First inject a failing signal creating an active gap
    fail_signal = LearningSignalCreate(
        course_id="CSC 301",
        topic="Dynamic Programming",
        subtopic="Memoization Table",
        signal_type="quiz_attempt",
        score=0,
        max_score=1,
        latency_ms=15000,
        confidence_level=2,
    )
    SignalIngestionService.record_signal(db_session, "student_02", fail_signal)

    active_gap = (
        db_session.query(LearningGap)
        .filter(
            LearningGap.user_id == "student_02",
            LearningGap.subtopic == "Memoization Table",
            LearningGap.status == "active",
        )
        .first()
    )
    assert active_gap is not None

    # 2. Inject successive high-scoring fluent responses to achieve mastery
    for _ in range(3):
        pass_signal = LearningSignalCreate(
            course_id="CSC 301",
            topic="Dynamic Programming",
            subtopic="Memoization Table",
            signal_type="quiz_attempt",
            score=1,
            max_score=1,
            latency_ms=4500,  # Fast correct
            confidence_level=4,  # Confident
        )
        SignalIngestionService.record_signal(db_session, "student_02", pass_signal)

    # Verify gap was resolved
    resolved_gap = (
        db_session.query(LearningGap)
        .filter(
            LearningGap.user_id == "student_02",
            LearningGap.subtopic == "Memoization Table",
        )
        .first()
    )
    assert resolved_gap.status == "resolved"
    assert resolved_gap.resolved_at is not None

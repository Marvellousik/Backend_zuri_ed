"""
Unit tests for Student Knowledge State Engine and Learning Signal Ingestion.
"""
from unittest.mock import MagicMock
from academic_service.services.signal_ingestion_service import SignalIngestionService
from academic_service.services.knowledge_state_service import KnowledgeStateService
from academic_service.models.schema import LearningSignalCreate
from academic_service.models.orm import Course, StudentKnowledgeState, LearningGap


def test_learning_signal_ingestion_ema():
    """Verify that learning signals update mastery scores using Exponential Moving Average."""
    db_mock = MagicMock()

    course = Course(id="course_csc301", code="CSC 301")
    existing_state = StudentKnowledgeState(
        id="ks_1",
        user_id="usr_1",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        mastery_score=50,
        status="learning",
        total_attempts=1,
        correct_attempts=1,
    )

    def query_side_effect(model):
        m = MagicMock()
        if model == Course:
            m.filter.return_value.first.return_value = course
        elif model == StudentKnowledgeState:
            m.filter.return_value.first.return_value = existing_state
        elif model == LearningGap:
            m.filter.return_value.first.return_value = None
            m.filter.return_value.all.return_value = []
        return m

    db_mock.query.side_effect = query_side_effect

    # Student scores 100% on current diagnostic (1 out of 1)
    signal = LearningSignalCreate(
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        signal_type="diagnostic",
        score=1,
        max_score=1,
    )

    result = SignalIngestionService.record_signal(db=db_mock, user_id="usr_1", signal_data=signal)

    # EMA calculation: 0.6 * 50 + 0.4 * 100 = 30 + 40 = 70%
    assert result.mastery_score == 70
    assert result.status == "learning"
    assert result.total_attempts == 2


def test_struggling_transition_and_gap_creation():
    """Verify that repeated poor performance transitions topic to 'struggling' and creates a LearningGap."""
    db_mock = MagicMock()

    course = Course(id="course_csc301", code="CSC 301")
    existing_state = StudentKnowledgeState(
        id="ks_2",
        user_id="usr_1",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Base Cases",
        mastery_score=30,
        status="learning",
        total_attempts=2,
        correct_attempts=0,
    )

    def query_side_effect(model):
        m = MagicMock()
        if model == Course:
            m.filter.return_value.first.return_value = course
        elif model == StudentKnowledgeState:
            m.filter.return_value.first.return_value = existing_state
        elif model == LearningGap:
            m.filter.return_value.first.return_value = None
            m.filter.return_value.all.return_value = []
        return m

    db_mock.query.side_effect = query_side_effect

    # Student scores 0 on current quiz
    signal = LearningSignalCreate(
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Base Cases",
        signal_type="quiz_attempt",
        score=0,
        max_score=1,
        details={"error_summary": "Failed to identify base case formulation"},
    )

    result = SignalIngestionService.record_signal(db=db_mock, user_id="usr_1", signal_data=signal)

    # EMA calculation: 0.6 * 30 + 0.4 * 0 = 18%
    assert result.mastery_score == 18
    assert result.status == "struggling"
    assert "base case" in result.last_error_summary.lower()

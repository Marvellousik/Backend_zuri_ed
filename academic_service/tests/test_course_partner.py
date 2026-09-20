"""
Unit tests for Phase 3: Mode 2 — Course Academic Partner (Student ↔ Lexi).
Validates intent classification, lecture matching, provenance citations, AI gateway integration,
interactive UI widget payload formulation, and partner conversation memory persistence.
"""
import pytest
from datetime import datetime, timezone
from unittest.mock import MagicMock, patch, AsyncMock
from sqlalchemy.orm import Session

from academic_service.services.course_partner_service import CoursePartnerService
from academic_service.models.orm import (
    Course,
    Lecture,
    LearningGap,
    StudentKnowledgeState,
    PartnerConversation,
)
from academic_service.models.schema import (
    PartnerChatRequest,
    WeakTopicDTO,
    FlashcardWidgetDTO,
    QuizWidgetDTO,
    ConceptWidgetDTO,
)


# ==============================================================================
# 1. Intent Classifier Tests
# ==============================================================================

def test_intent_classifier_flashcards():
    """Verify that prompts requesting flashcards are classified with correct count."""
    intent, params = CoursePartnerService.classify_intent("make 5 flashcards for Optimal Substructure")
    assert intent == "FLASHCARD_GENERATION"
    assert params["count"] == 5

    intent2, params2 = CoursePartnerService.classify_intent("give me a study deck on Lecture 14")
    assert intent2 == "FLASHCARD_GENERATION"
    assert params2["count"] == 5
    assert params2["lecture_number"] == 14


def test_intent_classifier_diagnostic_quiz():
    """Verify that quiz and test prompts are classified as DIAGNOSTIC_QUIZ."""
    intent, params = CoursePartnerService.classify_intent("quiz me on Lecture 14 with 3 questions")
    assert intent == "DIAGNOSTIC_QUIZ"
    assert params["count"] == 3
    assert params["lecture_number"] == 14

    intent2, params2 = CoursePartnerService.classify_intent("give me practice questions on base cases")
    assert intent2 == "DIAGNOSTIC_QUIZ"
    assert params2["count"] == 3


def test_intent_classifier_formula_card():
    """Verify that requests for formulas and equations are classified as FORMULA_CARD."""
    intent, params = CoursePartnerService.classify_intent("formula sheet for recurrence relations")
    assert intent == "FORMULA_CARD"

    intent2, _ = CoursePartnerService.classify_intent("give me the formula card for 0/1 knapsack")
    assert intent2 == "FORMULA_CARD"


def test_intent_classifier_concept_breakdown():
    """Verify that comparison and breakdown requests are classified as CONCEPT_BREAKDOWN."""
    intent, _ = CoursePartnerService.classify_intent("concept breakdown of Dynamic Programming vs Greedy")
    assert intent == "CONCEPT_BREAKDOWN"

    intent2, _ = CoursePartnerService.classify_intent("give me a summary table comparing Memoization and Tabulation")
    assert intent2 == "CONCEPT_BREAKDOWN"


def test_intent_classifier_explanation():
    """Verify that conceptual explanation queries are classified as EXPLANATION."""
    intent, _ = CoursePartnerService.classify_intent("explain memoization using the lecturer's example")
    assert intent == "EXPLANATION"

    intent2, _ = CoursePartnerService.classify_intent("how does optimal substructure work in shortest path?")
    assert intent2 == "EXPLANATION"


def test_intent_classifier_general_tutor():
    """Verify open-ended tutoring and guidance fall back to GENERAL_TUTOR."""
    intent, _ = CoursePartnerService.classify_intent("how should I prepare for the upcoming midterms?")
    assert intent == "GENERAL_TUTOR"

    intent2, _ = CoursePartnerService.classify_intent("Hello Lexi, what are we studying today?")
    assert intent2 == "GENERAL_TUTOR"


# ==============================================================================
# 2. Lecturer Provenance Citation & Matching Tests
# ==============================================================================

def test_lecturer_provenance_citations():
    """Verify matching canonical lectures and generating clean provenance citations."""
    lec12 = Lecture(
        id="lec_12",
        course_id="course_csc301",
        lecture_number=12,
        title="Introduction to Dynamic Programming",
        topics_covered=["Memoization", "Fibonacci Subproblems"],
        summary_text="Top-down memoization vs bottom-up tabulation.",
    )
    lec14 = Lecture(
        id="lec_14",
        course_id="course_csc301",
        lecture_number=14,
        title="Optimal Substructure & Base Cases",
        topics_covered=["Optimal Substructure", "Base Case Formulations", "Recurrence Relations"],
        summary_text="Proving optimal substructure with cut-and-paste argument.",
    )

    course = Course(
        id="course_csc301",
        code="CSC 301",
        title="Data Structures & Algorithms",
        lectures=[lec12, lec14],
    )

    # 1. Exact lecture match by number
    matched = CoursePartnerService.find_relevant_lectures(course, "explain lecture 14", lecture_number=14)
    assert len(matched) == 1
    assert matched[0].lecture_number == 14

    # 2. Topic keyword matching
    matched_topic = CoursePartnerService.find_relevant_lectures(course, "how does memoization work?")
    assert len(matched_topic) >= 1
    assert matched_topic[0].lecture_number == 12

    # 3. Citation formatting
    citations = CoursePartnerService.format_provenance_citations(course, [lec14])
    assert len(citations) == 1
    assert "[CSC 301 · Lecture 14: Optimal Substructure & Base Cases, Slide" in citations[0]


# ==============================================================================
# 3. End-to-End Course Partner Chat & Widget Generation Tests
import asyncio

# ==============================================================================
# 3. End-to-End Course Partner Chat & Widget Generation Tests
# ==============================================================================

def test_chat_flashcard_generation_widget():
    """Verify CoursePartnerService.chat emits structured flashcard widget and persists history."""
    db_mock = MagicMock(spec=Session)

    lec14 = Lecture(
        id="lec_14",
        course_id="course_csc301",
        lecture_number=14,
        title="Optimal Substructure & Base Cases",
        topics_covered=["Optimal Substructure", "Base Case Formulations"],
    )

    course = Course(
        id="course_csc301",
        institution_id="veritas_uni",
        code="CSC 301",
        title="Data Structures & Algorithms",
        level=300,
        lectures=[lec14],
        syllabus=[{"topic": "Dynamic Programming", "subtopics": ["Optimal Substructure"]}],
    )

    def query_side_effect(model):
        m = MagicMock()
        if model == Course:
            m.filter.return_value.first.return_value = course
        elif model == StudentKnowledgeState:
            m.filter.return_value.first.return_value = None
            m.filter.return_value.count.return_value = 1
        elif model == LearningGap:
            m.filter.return_value.all.return_value = []
        elif model == PartnerConversation:
            m.filter.return_value.all.return_value = []
        return m

    db_mock.query.side_effect = query_side_effect

    weak_topic = WeakTopicDTO(
        course_id="course_csc301",
        course_code="CSC 301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        mastery_score=30,
        status="struggling",
        reason="Repeatedly misses base cases",
    )

    req = PartnerChatRequest(
        course_id="CSC 301",
        topic="Optimal Substructure",
        message="make 5 flashcards for Optimal Substructure",
        session_id="sess_test_123",
    )

    with patch("academic_service.services.course_partner_service.KnowledgeStateService.get_weakest_topics", return_value=[weak_topic]), \
         patch("academic_service.services.course_partner_service.KnowledgeStateService.initialize_student_course_state"):

        response = asyncio.run(
            CoursePartnerService.chat(
                db=db_mock,
                user_id="usr_demo",
                request=req,
            )
        )

        assert response.intent == "FLASHCARD_GENERATION"
        assert response.widget_type == "flashcard_deck"
        assert response.widget_data is not None
        assert response.widget_data["count"] >= 3
        assert len(response.widget_data["cards"]) >= 3
        assert response.course_code == "CSC 301"
        assert response.session_id == "sess_test_123"
        assert len(response.provenance_citations) > 0
        assert "CSC 301" in response.provenance_citations[0]

        # Verify DB added 2 conversation records (user message and assistant response)
        assert db_mock.add.call_count >= 2
        assert db_mock.commit.called


def test_chat_diagnostic_quiz_widget():
    """Verify CoursePartnerService.chat emits structured multiple-choice quiz widget."""
    db_mock = MagicMock(spec=Session)

    course = Course(
        id="course_csc301",
        institution_id="veritas_uni",
        code="CSC 301",
        title="Data Structures & Algorithms",
        level=300,
        lectures=[],
        syllabus=[{"topic": "Dynamic Programming", "subtopics": ["Optimal Substructure"]}],
    )

    def query_side_effect(model):
        m = MagicMock()
        if model == Course:
            m.filter.return_value.first.return_value = course
        elif model == StudentKnowledgeState:
            m.filter.return_value.count.return_value = 1
        elif model == LearningGap:
            m.filter.return_value.all.return_value = []
        return m

    db_mock.query.side_effect = query_side_effect

    req = PartnerChatRequest(
        course_id="course_csc301",
        topic="Dynamic Programming",
        message="quiz me on Lecture 14 with 3 questions",
        session_id="sess_quiz_1",
    )

    with patch("academic_service.services.course_partner_service.KnowledgeStateService.get_weakest_topics", return_value=[]), \
         patch("academic_service.services.course_partner_service.KnowledgeStateService.initialize_student_course_state"):

        response = asyncio.run(
            CoursePartnerService.chat(
                db=db_mock,
                user_id="usr_demo",
                request=req,
            )
        )

        assert response.intent == "DIAGNOSTIC_QUIZ"
        assert response.widget_type == "quiz_card"
        assert response.widget_data is not None
        assert response.widget_data["quiz_type"] == "multiple_choice"
        assert response.widget_data["count"] == 3
        assert len(response.widget_data["questions"]) == 3

        first_q = response.widget_data["questions"][0]
        assert "question" in first_q
        assert "options" in first_q
        assert "correct_answer" in first_q


def test_chat_concept_breakdown_widget():
    """Verify CoursePartnerService.chat emits concept breakdown with formulas and takeaways."""
    db_mock = MagicMock(spec=Session)

    course = Course(
        id="course_csc301",
        institution_id="veritas_uni",
        code="CSC 301",
        title="Data Structures & Algorithms",
        level=300,
        lectures=[],
    )

    def query_side_effect(model):
        m = MagicMock()
        if model == Course:
            m.filter.return_value.first.return_value = course
        elif model == LearningGap:
            m.filter.return_value.all.return_value = []
        return m

    db_mock.query.side_effect = query_side_effect

    req = PartnerChatRequest(
        course_id="course_csc301",
        message="concept breakdown of Dynamic Programming recurrence formulations",
    )

    with patch("academic_service.services.course_partner_service.KnowledgeStateService.get_weakest_topics", return_value=[]), \
         patch("academic_service.services.course_partner_service.KnowledgeStateService.initialize_student_course_state"):

        response = asyncio.run(
            CoursePartnerService.chat(
                db=db_mock,
                user_id="usr_demo",
                request=req,
            )
        )

        assert response.intent == "CONCEPT_BREAKDOWN"
        assert response.widget_type == "concept_card"
        assert response.widget_data is not None
        assert len(response.widget_data["concepts"]) >= 2
        assert "Optimal Substructure" in [c["term"] for c in response.widget_data["concepts"]]


def test_chat_explanation_with_weak_topic_awareness():
    """Verify that pedagogical explanations reference the student's weak topic status."""
    db_mock = MagicMock(spec=Session)

    course = Course(
        id="course_csc301",
        institution_id="veritas_uni",
        code="CSC 301",
        title="Data Structures & Algorithms",
        level=300,
        lectures=[],
    )

    def query_side_effect(model):
        m = MagicMock()
        if model == Course:
            m.filter.return_value.first.return_value = course
        elif model == LearningGap:
            m.filter.return_value.all.return_value = []
        return m

    db_mock.query.side_effect = query_side_effect

    weak_topic = WeakTopicDTO(
        course_id="course_csc301",
        course_code="CSC 301",
        topic="Dynamic Programming",
        subtopic="Base Cases",
        mastery_score=25,
        status="struggling",
        reason="Repeatedly misses base cases",
    )

    req = PartnerChatRequest(
        course_id="course_csc301",
        topic="Base Cases",
        message="explain why base cases are required in dynamic programming",
    )

    with patch("academic_service.services.course_partner_service.KnowledgeStateService.get_weakest_topics", return_value=[weak_topic]), \
         patch("academic_service.services.course_partner_service.KnowledgeStateService.initialize_student_course_state"):

        response = asyncio.run(
            CoursePartnerService.chat(
                db=db_mock,
                user_id="usr_demo",
                request=req,
            )
        )

        assert response.intent == "EXPLANATION"
        assert "base cases" in response.message.lower()
        assert "Base Cases" in response.weak_topics_referenced


def test_partner_history_retrieval():
    """Verify retrieving partner conversation history in chronological order."""
    db_mock = MagicMock(spec=Session)

    course = Course(id="course_csc301", code="CSC 301", title="Data Structures")

    conv1 = PartnerConversation(
        id="p1",
        user_id="usr_demo",
        course_id="course_csc301",
        session_id="sess_1",
        role="user",
        content="make 5 flashcards for Optimal Substructure",
        created_at=datetime(2026, 9, 1, 10, 0, 0, tzinfo=timezone.utc),
    )
    conv2 = PartnerConversation(
        id="p2",
        user_id="usr_demo",
        course_id="course_csc301",
        session_id="sess_1",
        role="assistant",
        content="Here is your 5-card study deck...",
        widget_type="flashcard_deck",
        widget_data={"count": 5},
        provenance_citations=["[CSC 301 · Lecture 14, Slide 5]"],
        created_at=datetime(2026, 9, 1, 10, 0, 2, tzinfo=timezone.utc),
    )

    def query_side_effect(model):
        m = MagicMock()
        if model == Course:
            m.filter.return_value.first.return_value = course
        elif model == PartnerConversation:
            m.filter.return_value.filter.return_value.order_by.return_value.limit.return_value.all.return_value = [conv1, conv2]
            m.filter.return_value.order_by.return_value.limit.return_value.all.return_value = [conv1, conv2]
        return m

    db_mock.query.side_effect = query_side_effect

    history = CoursePartnerService.get_history(
        db=db_mock,
        user_id="usr_demo",
        course_id="course_csc301",
        session_id="sess_1",
    )

    assert history.course_id == "course_csc301"
    assert history.course_code == "CSC 301"
    assert len(history.messages) == 2
    assert history.messages[0].role == "user"
    assert history.messages[1].role == "assistant"
    assert history.messages[1].widget_type == "flashcard_deck"
    assert history.messages[1].provenance_citations == ["[CSC 301 · Lecture 14, Slide 5]"]


def test_course_not_found_handling():
    """Verify that querying an unknown course raises ValueError."""
    db_mock = MagicMock(spec=Session)
    db_mock.query(Course).filter().first.return_value = None

    req = PartnerChatRequest(
        course_id="NON_EXISTENT_COURSE",
        message="quiz me on algorithms",
    )

    with pytest.raises(ValueError) as exc_info:
        asyncio.run(CoursePartnerService.chat(db=db_mock, user_id="usr_demo", request=req))

    assert "not found" in str(exc_info.value).lower()


# ==============================================================================
# 4. FastAPI REST Endpoint Integration Tests
# ==============================================================================

from fastapi.testclient import TestClient
from academic_service.cmd.main import app
from academic_service.storage.database import get_db

def test_api_course_partner_chat():
    """Verify POST /api/v1/academic/courses/{course_id}/partner/chat returns 200 with widgets."""
    db_mock = MagicMock(spec=Session)

    course = Course(
        id="course_csc301",
        institution_id="veritas_uni",
        code="CSC 301",
        title="Data Structures & Algorithms",
        level=300,
        lectures=[],
    )

    def query_side_effect(model):
        m = MagicMock()
        if model == Course:
            m.filter.return_value.first.return_value = course
        elif model == LearningGap:
            m.filter.return_value.all.return_value = []
        elif model == PartnerConversation:
            m.filter.return_value.all.return_value = []
        return m

    db_mock.query.side_effect = query_side_effect

    app.dependency_overrides[get_db] = lambda: db_mock
    client = TestClient(app)

    payload = {
        "topic": "Optimal Substructure",
        "message": "make 5 flashcards for Optimal Substructure",
        "session_id": "sess_api_test",
    }

    with patch("academic_service.services.course_partner_service.KnowledgeStateService.get_weakest_topics", return_value=[]), \
         patch("academic_service.services.course_partner_service.KnowledgeStateService.initialize_student_course_state"):

        resp = client.post(
            "/api/v1/academic/courses/CSC%20301/partner/chat",
            json=payload,
            headers={"X-User-ID": "usr_demo_student"},
        )

        assert resp.status_code == 200
        data = resp.json()
        assert data["intent"] == "FLASHCARD_GENERATION"
        assert data["widget_type"] == "flashcard_deck"
        assert data["course_code"] == "CSC 301"
        assert data["session_id"] == "sess_api_test"
        assert len(data["provenance_citations"]) > 0

    app.dependency_overrides.clear()


def test_api_course_partner_history():
    """Verify GET /api/v1/academic/courses/{course_id}/partner/history returns previous turns."""
    db_mock = MagicMock(spec=Session)

    course = Course(id="course_csc301", code="CSC 301", title="Data Structures")
    conv1 = PartnerConversation(
        id="p1",
        user_id="usr_demo_student",
        course_id="course_csc301",
        session_id="sess_hist_1",
        role="user",
        content="quiz me on dynamic programming",
        created_at=datetime(2026, 9, 1, 11, 0, 0, tzinfo=timezone.utc),
    )

    def query_side_effect(model):
        m = MagicMock()
        if model == Course:
            m.filter.return_value.first.return_value = course
        elif model == PartnerConversation:
            m.filter.return_value.filter.return_value.order_by.return_value.limit.return_value.all.return_value = [conv1]
            m.filter.return_value.order_by.return_value.limit.return_value.all.return_value = [conv1]
        return m

    db_mock.query.side_effect = query_side_effect

    app.dependency_overrides[get_db] = lambda: db_mock
    client = TestClient(app)

    resp = client.get(
        "/api/v1/academic/courses/CSC%20301/partner/history?session_id=sess_hist_1",
        headers={"X-User-ID": "usr_demo_student"},
    )

    assert resp.status_code == 200
    data = resp.json()
    assert data["course_code"] == "CSC 301"
    assert len(data["messages"]) == 1
    assert data["messages"][0]["content"] == "quiz me on dynamic programming"

    app.dependency_overrides.clear()



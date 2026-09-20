"""
Unit tests for Phases 32 & 33: Lecturer Intelligence Dashboard & Pedagogical Intervention System.
Verifies anonymized cohort analytics (zero student PII), question clusters, lecture resonance,
and targeted intervention dispatch into struggling students' Today timelines.
"""
from datetime import datetime, timezone
import pytest
from sqlalchemy import create_engine, event
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from academic_service.models.orm import (
    Base,
    Course,
    Lecture,
    StudentEnrollment,
    StudentKnowledgeState,
    LearningGap,
    ProactiveIntervention,
)
from academic_service.services.lecturer_signal_service import LecturerSignalService


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
        # Seed course and lecture
        course = Course(
            id="course_csc301",
            institution_id="inst_veritas",
            department_id="dept_cs",
            code="CSC 301",
            title="Data Structures and Algorithms",
            syllabus=[
                {"topic": "Dynamic Programming", "subtopics": ["Optimal Substructure", "Memoization"]},
                {"topic": "Graph Algorithms", "subtopics": ["Dijkstra", "Bellman-Ford"]},
            ],
        )
        lec1 = Lecture(
            id="lec_14",
            course_id="course_csc301",
            lecture_number=14,
            title="Dynamic Programming and Optimal Substructure",
            topics_covered=["Optimal Substructure", "Memoization"],
        )
        session.add_all([course, lec1])
        session.commit()
        yield session
    finally:
        session.close()


def test_get_cohort_intelligence_dashboard_anonymized(db_session):
    """
    Verify cohort dashboard returns anonymized misconceptions, question clusters,
    and lecture resonance with ZERO student PII.
    """
    # Seed 3 enrollments
    e1 = StudentEnrollment(id="enr_1", user_id="student_alice", course_id="course_csc301")
    e2 = StudentEnrollment(id="enr_2", user_id="student_bob", course_id="course_csc301")
    e3 = StudentEnrollment(id="enr_3", user_id="student_charlie", course_id="course_csc301")

    # Seed gaps
    g1 = LearningGap(
        id="gap_1",
        user_id="student_alice",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        gap_description="Repeatedly misses base cases in recurrence formulations.",
        severity="high",
        status="active",
    )
    g2 = LearningGap(
        id="gap_2",
        user_id="student_bob",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        gap_description="Repeatedly misses base cases in recurrence formulations.",
        severity="high",
        status="active",
    )

    # Seed knowledge states
    ks1 = StudentKnowledgeState(
        id="ks_1",
        user_id="student_alice",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        mastery_score=35,
        status="struggling",
    )
    ks2 = StudentKnowledgeState(
        id="ks_2",
        user_id="student_bob",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        mastery_score=40,
        status="struggling",
    )
    ks3 = StudentKnowledgeState(
        id="ks_3",
        user_id="student_charlie",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        mastery_score=90,
        status="mastered",
    )

    db_session.add_all([e1, e2, e3, g1, g2, ks1, ks2, ks3])
    db_session.commit()

    dashboard = LecturerSignalService.get_cohort_intelligence_dashboard(
        db=db_session,
        course_id="CSC 301",
    )

    assert dashboard["course_code"] == "CSC 301"
    assert dashboard["total_enrolled"] == 3

    # Check top misconceptions
    assert len(dashboard["top_misconceptions"]) > 0
    top_m = dashboard["top_misconceptions"][0]
    assert "misses base cases" in top_m["misconception"]
    assert top_m["affected_students_count"] == 2
    assert top_m["percentage_of_cohort"] == 66

    # Verify ZERO student PII in dashboard
    dashboard_str = str(dashboard)
    assert "student_alice" not in dashboard_str
    assert "student_bob" not in dashboard_str
    assert "student_charlie" not in dashboard_str

    # Check lecture resonance metrics
    assert len(dashboard["lecture_resonance"]) >= 1
    lec_res = dashboard["lecture_resonance"][0]
    assert lec_res["lecture_number"] == 14
    assert "resonance_score" in lec_res


def test_dispatch_pedagogical_intervention_targeted_to_struggling_students(db_session):
    """
    Verify that dispatching an intervention creates cards ONLY for students struggling in the target topic.
    """
    e1 = StudentEnrollment(id="enr_s1", user_id="student_struggling_1", course_id="course_csc301")
    e2 = StudentEnrollment(id="enr_s2", user_id="student_struggling_2", course_id="course_csc301")
    e3 = StudentEnrollment(id="enr_s3", user_id="student_mastered", course_id="course_csc301")

    # Struggling students in DP
    ks1 = StudentKnowledgeState(
        id="ks_s1",
        user_id="student_struggling_1",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        mastery_score=25,
        status="struggling",
    )
    ks2 = StudentKnowledgeState(
        id="ks_s2",
        user_id="student_struggling_2",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        mastery_score=30,
        status="struggling",
    )
    # Mastered student in DP
    ks3 = StudentKnowledgeState(
        id="ks_s3",
        user_id="student_mastered",
        course_id="course_csc301",
        topic="Dynamic Programming",
        subtopic="Optimal Substructure",
        mastery_score=95,
        status="mastered",
    )

    db_session.add_all([e1, e2, e3, ks1, ks2, ks3])
    db_session.commit()

    revision_pack = {
        "title": "Recurrence Invariants & Base Cases Masterclass",
        "key_points": ["Always test n=0 and n=1", "Tabulate states in topological order"],
        "practice_question_ids": ["q_base_cases_1"],
    }

    result = LecturerSignalService.dispatch_pedagogical_intervention(
        db=db_session,
        course_id="CSC 301",
        lecturer_id="lecturer_dr_okafor",
        topic="Dynamic Programming",
        title="Extra Revision: Base Cases & State Invariants",
        revision_pack=revision_pack,
    )

    assert result["status"] == "dispatched"
    assert result["targeted_students_count"] == 2
    assert result["card_type"] == "lecturer_revision_pack"
    assert result["priority"] == 1

    # Verify ProactiveIntervention records in DB
    interventions = db_session.query(ProactiveIntervention).all()
    assert len(interventions) == 2

    recipient_user_ids = {i.user_id for i in interventions}
    assert "student_struggling_1" in recipient_user_ids
    assert "student_struggling_2" in recipient_user_ids
    assert "student_mastered" not in recipient_user_ids

    # Check intervention payload properties
    sample_int = interventions[0]
    assert sample_int.card_type == "lecturer_revision_pack"
    assert sample_int.payload["priority"] == 1
    assert sample_int.payload["lecturer_id"] == "lecturer_dr_okafor"
    assert "Extra Revision" in sample_int.title

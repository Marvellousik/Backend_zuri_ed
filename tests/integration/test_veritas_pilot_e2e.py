"""
Integration Test Suite: Veritas Pilot & Contract Readiness (Phase 40).
Comprehensive end-to-end integration testing for the Veritas University production pilot:
1. Seed Veritas University academic context graph (Hierarchy, Calendar, Courses, Offerings, Timetables).
2. Audit institutional pilot readiness to ensure 100% readiness score and zero blocking items.
3. Verify student cohort enrollment and dynamic 'Today' timeline generation.
4. Process lecture transcript through the Lecture Intelligence Pipeline (pedagogical notes + study deck).
5. Student completes adaptive practice session with latency and confidence telemetry, resolving learning gaps.
6. Lecturer monitors cohort intelligence dashboard and dispatches targeted pedagogical intervention.
7. Institutional administrator logs pilot deployment actions and verifies audit trail.
8. Departmental cost intelligence calculations ensure budget compliance and nominal throttle status.
"""
import pytest
from datetime import datetime, timezone
from sqlalchemy import create_engine, event
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from academic_service.models.orm import (
    Base,
    Institution,
    Course,
    Lecture,
    CourseOffering,
    StudentEnrollment,
    StudentKnowledgeState,
    LearningGap,
    ProactiveIntervention,
)
from ai_service.storage.models import Base as AIBase
from academic_service.seeds.veritas_seed import seed_veritas_university
from academic_service.services.pilot_readiness_service import PilotReadinessService
from academic_service.services.lecture_ingestion_service import LectureIngestionService
from academic_service.services.proactive_engine import ProactiveEngine
from academic_service.services.adaptive_practice_service import AdaptivePracticeService
from academic_service.services.lecturer_signal_service import LecturerSignalService
from academic_service.services.admin_service import AdminService
from academic_service.services.cost_intelligence_service import CostIntelligenceService


@pytest.fixture
def db_session():
    """In-memory SQLite database session fixture with attached academic and ai schemas."""
    engine = create_engine(
        "sqlite://",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )

    @event.listens_for(engine, "connect")
    def do_connect(dbapi_connection, connection_record):
        cursor = dbapi_connection.cursor()
        cursor.execute("ATTACH DATABASE ':memory:' AS academic")
        cursor.execute("ATTACH DATABASE ':memory:' AS ai")
        cursor.close()

    Base.metadata.create_all(bind=engine)
    AIBase.metadata.create_all(bind=engine)

    SessionLocal = sessionmaker(bind=engine)
    session = SessionLocal()
    try:
        yield session
    finally:
        session.close()


def test_veritas_pilot_readiness_audit_100_percent(db_session):
    """Step 1 & 2: Seed Veritas University and assert 100% pilot readiness score."""
    seed_veritas_university(db_session)

    report = PilotReadinessService.audit_institution_pilot_readiness(db_session, "veritas_uni")

    assert report["is_pilot_ready"] is True
    assert report["readiness_score_percentage"] == 100
    assert len(report["blocking_items"]) == 0
    assert report["checklist"]["hierarchy"]["status"] == "PASSED"
    assert report["checklist"]["calendar"]["status"] == "PASSED"
    assert report["checklist"]["syllabi"]["status"] == "PASSED"
    assert report["checklist"]["course_offerings"]["status"] == "PASSED"
    assert report["checklist"]["timetable_slots"]["status"] == "PASSED"
    assert report["checklist"]["enrolled_cohorts"]["status"] == "PASSED"


def test_veritas_pilot_student_enrollment_and_today_timeline(db_session):
    """Step 3: Student enrollment and proactive 'Today' timeline synthesis."""
    seed_veritas_university(db_session)

    # 1. Verify demo student enrollment
    enr = (
        db_session.query(StudentEnrollment)
        .filter(
            StudentEnrollment.user_id == "usr_demo_student_veritas",
            StudentEnrollment.course_id == "course_csc301_veritas",
        )
        .first()
    )
    assert enr is not None
    assert enr.course_offering_id == "off_csc301_veritas"
    assert enr.status == "active"

    # 2. Add second pilot student to cohort
    std2_enr = StudentEnrollment(
        id="enr_std2_csc301",
        user_id="usr_pilot_student_02",
        course_id="course_csc301_veritas",
        course_offering_id="off_csc301_veritas",
        semester="2025/2026_FIRST",
        status="active",
    )
    db_session.add(std2_enr)
    db_session.commit()

    # 3. Generate Today timeline for demo student
    timeline = ProactiveEngine.generate_today_timeline(
        db=db_session,
        user_id="usr_demo_student_veritas",
    )
    assert timeline is not None
    assert isinstance(timeline.cards, list)
    assert len(timeline.cards) > 0


@pytest.mark.anyio
async def test_veritas_pilot_lecture_intelligence_pipeline(db_session):
    """Step 4: Lecture transcript ingestion, pedagogical notes, and companion study deck generation."""
    seed_veritas_university(db_session)

    transcript = """
    [00:00:10] Dr. Okafor: Good morning class. Welcome to CSC 301 Data Structures and Algorithms.
    [00:01:45] Dr. Okafor: Today we are dissecting Optimal Substructure in Dynamic Programming.
    [00:06:12] Dr. Okafor: An optimal solution to any dynamic programming problem incorporates optimal solutions to related subproblems.
    [00:10:30] Dr. Okafor: When designing recurrence relations, base case formulations are essential to guarantee convergence.
    [00:16:00] Dr. Okafor: Next week we will contrast Memoization with Bottom-Up Tabulation.
    """

    result = await LectureIngestionService.process_lecture_recording_or_transcript(
        db=db_session,
        lecture_id="lec_csc301_opt_sub",
        transcript_text=transcript,
    )

    assert result["status"] == "processed"
    assert "pedagogical_notes" in result
    assert len(result["pedagogical_notes"]) > 100
    assert "study_deck" in result
    assert result["study_deck"]["card_count"] > 0
    assert result["study_deck"]["question_count"] > 0

    # Verify database update
    lec = db_session.query(Lecture).filter(Lecture.id == "lec_csc301_opt_sub").first()
    assert lec.is_processed is True
    assert "Optimal Substructure" in lec.topics_covered
    assert lec.summary_text is not None


def test_veritas_pilot_adaptive_practice_and_gap_resolution(db_session):
    """Step 5: Targeted adaptive practice session with telemetry and knowledge gap auto-resolution."""
    seed_veritas_university(db_session)

    # Verify initial struggling knowledge state & active gap
    initial_ks = (
        db_session.query(StudentKnowledgeState)
        .filter(
            StudentKnowledgeState.user_id == "usr_demo_student_veritas",
            StudentKnowledgeState.subtopic == "Optimal Substructure",
        )
        .first()
    )
    assert initial_ks is not None
    assert initial_ks.mastery_score == 30
    assert initial_ks.status == "struggling"
    initial_score = int(initial_ks.mastery_score)

    initial_gap = (
        db_session.query(LearningGap)
        .filter(
            LearningGap.user_id == "usr_demo_student_veritas",
            LearningGap.subtopic == "Optimal Substructure",
            LearningGap.status == "active",
        )
        .first()
    )
    assert initial_gap is not None

    # 1. Generate adaptive practice session
    session_data = AdaptivePracticeService.generate_adaptive_practice_session(
        db=db_session,
        user_id="usr_demo_student_veritas",
        course_id="course_csc301_veritas",
        gap_id=initial_gap.id,
    )
    assert session_data["target_subtopic"] == "Optimal Substructure"
    assert session_data["total_questions"] >= 2

    # 2. Submit high-mastery practice attempt 1 (progression from 30% -> 62%)
    attempt_res_1 = AdaptivePracticeService.submit_practice_attempt(
        db=db_session,
        user_id="usr_demo_student_veritas",
        course_id="course_csc301_veritas",
        attempt_data={
            "session_id": session_data["session_id"],
            "topic": session_data["target_topic"],
            "subtopic": session_data["target_subtopic"],
            "gap_id": initial_gap.id,
            "answers": {
                "q_1": "A",
                "q_2": "B",
                "q_3": "State representation S(i, j) with base condition 0 and optimal transition recurrence.",
            },
            "theory_points": 5,
            "latency_ms": 5000,
            "confidence_level": 5,
        },
    )
    assert attempt_res_1["percentage"] >= 85
    assert attempt_res_1["updated_mastery_score"] > initial_score

    # 3. Submit follow-up practice attempt 2 (verified mastery 62% -> 79% >= 75%, auto-resolving gap)
    attempt_res_2 = AdaptivePracticeService.submit_practice_attempt(
        db=db_session,
        user_id="usr_demo_student_veritas",
        course_id="course_csc301_veritas",
        attempt_data={
            "session_id": f"{session_data['session_id']}_followup",
            "topic": session_data["target_topic"],
            "subtopic": session_data["target_subtopic"],
            "gap_id": initial_gap.id,
            "answers": {
                "q_1": "A",
                "q_2": "B",
                "q_3": "State representation S(i, j) with base condition 0 and optimal transition recurrence.",
            },
            "theory_points": 5,
            "latency_ms": 4500,
            "confidence_level": 5,
        },
    )
    assert attempt_res_2["updated_mastery_score"] >= 75
    assert attempt_res_2["gap_resolved"] is True

    # Check updated database gap status
    db_session.refresh(initial_gap)
    assert initial_gap.status == "resolved"


def test_veritas_pilot_lecturer_dashboard_and_intervention(db_session):
    """Step 6: Lecturer cohort intelligence dashboard and targeted pedagogical intervention dispatch."""
    seed_veritas_university(db_session)

    # 1. View cohort intelligence dashboard
    dashboard = LecturerSignalService.get_cohort_intelligence_dashboard(
        db=db_session,
        course_id="course_csc301_veritas",
    )
    assert dashboard["course_code"] == "CSC 301"
    assert dashboard["total_enrolled"] >= 1
    assert "question_clusters" in dashboard
    assert "lecture_resonance" in dashboard
    assert len(dashboard["lecture_resonance"]) >= 2

    # 2. Dispatch targeted pedagogical intervention
    intervention_res = LecturerSignalService.dispatch_pedagogical_intervention(
        db=db_session,
        course_id="course_csc301_veritas",
        lecturer_id="lect_dr_okafor_veritas",
        topic="Optimal Substructure",
        title="Dr. Okafor's Dynamic Programming Review Pack",
        revision_pack={
            "summary": "Detailed breakdown of optimal substructure and base case recurrence formulation.",
            "practice_problems": 3,
        },
    )
    assert intervention_res["status"] == "dispatched"
    assert intervention_res["targeted_students_count"] >= 1

    # Verify proactive intervention record created
    interventions = (
        db_session.query(ProactiveIntervention)
        .filter(ProactiveIntervention.course_id == "course_csc301_veritas")
        .all()
    )
    assert len(interventions) >= 1


def test_veritas_pilot_administration_and_cost_intelligence(db_session):
    """Step 7 & 8: Institutional administration audit logging and departmental cost intelligence."""
    seed_veritas_university(db_session)

    # 1. Administrative action logging
    audit_entry = AdminService.log_admin_action(
        db=db_session,
        institution_id="veritas_uni",
        actor_id="usr_admin_veritas",
        action="PILOT_CONTRACT_ACTIVATION",
        resource_type="institution",
        resource_id="veritas_uni",
        payload={
            "contract_type": "enterprise_pilot",
            "department": "Department of Computer Science",
            "student_capacity": 150,
            "pilot_start": "2026-08-01",
        },
    )
    assert audit_entry.id.startswith("audit_")

    # Verify audit log retrieval
    logs = AdminService.get_audit_logs(db_session, "veritas_uni")
    assert len(logs) >= 1
    assert any(l.action == "PILOT_CONTRACT_ACTIVATION" for l in logs)

    # 2. Departmental cost intelligence
    cost_report = CostIntelligenceService.get_department_cost_intelligence(
        db=db_session,
        institution_id="veritas_uni",
        department_id="dept_cs_veritas",
    )
    assert cost_report["department_id"] == "dept_cs_veritas"
    assert cost_report["throttle_status"] == "NOMINAL"
    assert cost_report["is_automated_throttling_active"] is False
    assert cost_report["learning_roi"]["total_mastery_points_gained"] > 0


@pytest.mark.anyio
async def test_veritas_pilot_e2e_full_lifecycle(db_session):
    """Complete 9-stage end-to-end integration lifecycle confirming Veritas University contract readiness."""
    # 1. Seed Veritas baseline
    seed_veritas_university(db_session)

    # 2. Verify pilot readiness
    readiness = PilotReadinessService.audit_institution_pilot_readiness(db_session, "veritas_uni")
    assert readiness["is_pilot_ready"] is True
    assert readiness["readiness_score_percentage"] == 100

    # 3. Verify student enrollment
    enr = db_session.query(StudentEnrollment).filter(StudentEnrollment.user_id == "usr_demo_student_veritas").first()
    assert enr is not None

    # 4. Ingest lecture transcript
    result = await LectureIngestionService.process_lecture_recording_or_transcript(
        db=db_session,
        lecture_id="lec_csc301_opt_sub",
        transcript_text="[00:01:00] Dr. Okafor: Today we master Optimal Substructure and Recurrence Relations.",
    )
    assert result["status"] == "processed"

    # 5. Student generates Today timeline
    timeline = ProactiveEngine.generate_today_timeline(db_session, "usr_demo_student_veritas")
    assert len(timeline.cards) > 0

    # 6. Student completes adaptive practice session (2-stage mastery progression resolving learning gap)
    session_data = AdaptivePracticeService.generate_adaptive_practice_session(
        db=db_session,
        user_id="usr_demo_student_veritas",
        course_id="course_csc301_veritas",
    )
    # Attempt 1: 30% -> 62%
    AdaptivePracticeService.submit_practice_attempt(
        db=db_session,
        user_id="usr_demo_student_veritas",
        course_id="course_csc301_veritas",
        attempt_data={
            "session_id": session_data["session_id"],
            "topic": session_data["target_topic"],
            "subtopic": session_data["target_subtopic"],
            "gap_id": session_data.get("target_gap_id"),
            "answers": {"q_1": "A", "q_2": "B", "q_3": "State representation S(i, j) with recurrence."},
            "theory_points": 5,
            "latency_ms": 5000,
            "confidence_level": 5,
        },
    )
    # Attempt 2: 62% -> 79% (resolves gap)
    attempt_res = AdaptivePracticeService.submit_practice_attempt(
        db=db_session,
        user_id="usr_demo_student_veritas",
        course_id="course_csc301_veritas",
        attempt_data={
            "session_id": f"{session_data['session_id']}_2",
            "topic": session_data["target_topic"],
            "subtopic": session_data["target_subtopic"],
            "gap_id": session_data.get("target_gap_id"),
            "answers": {"q_1": "A", "q_2": "B", "q_3": "State representation S(i, j) with recurrence."},
            "theory_points": 5,
            "latency_ms": 4500,
            "confidence_level": 5,
        },
    )
    assert attempt_res["gap_resolved"] is True

    # 7. Lecturer reviews dashboard and dispatches intervention
    dashboard = LecturerSignalService.get_cohort_intelligence_dashboard(db_session, "course_csc301_veritas")
    assert dashboard["total_enrolled"] >= 1

    intervention_res = LecturerSignalService.dispatch_pedagogical_intervention(
        db=db_session,
        course_id="course_csc301_veritas",
        lecturer_id="lect_dr_okafor_veritas",
        topic="Optimal Substructure",
        title="Midterm Preparation Pack",
        revision_pack={"summary": "Key review notes"},
    )
    assert intervention_res["status"] == "dispatched"

    # 8. Admin audit logging
    AdminService.log_admin_action(
        db=db_session,
        institution_id="veritas_uni",
        actor_id="usr_admin_veritas",
        action="VERITAS_PILOT_VERIFIED",
        resource_type="institution",
        resource_id="veritas_uni",
    )
    logs = AdminService.get_audit_logs(db_session, "veritas_uni")
    assert any(l.action == "VERITAS_PILOT_VERIFIED" for l in logs)

    # 9. Cost Intelligence
    cost_info = CostIntelligenceService.get_department_cost_intelligence(
        db=db_session,
        institution_id="veritas_uni",
        department_id="dept_cs_veritas",
    )
    assert cost_info["throttle_status"] == "NOMINAL"

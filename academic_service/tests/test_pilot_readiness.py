"""
Unit tests for Phase 36 (Institutional Administration Suite), Phase 37 (Cost Intelligence),
and Phase 39 (Pilot Readiness Audit).
"""
from datetime import date, time
import pytest
from sqlalchemy import create_engine, event
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from academic_service.models.orm import (
    Base,
    Institution,
    Faculty,
    Department,
    Program,
    AcademicSession,
    Semester,
    Course,
    CourseOffering,
    CourseSchedule,
    StudentEnrollment,
    StudentKnowledgeState,
    LearningSignal,
)
from academic_service.services.admin_service import AdminService
from academic_service.services.cost_intelligence_service import CostIntelligenceService
from academic_service.services.pilot_readiness_service import PilotReadinessService


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
        yield session
    finally:
        session.close()


def test_audit_pilot_readiness_incomplete(db_session):
    """Verify that an institution with missing hierarchy and offerings is flagged as not ready."""
    inst = Institution(id="inst_incomplete", name="New University", code="NU")
    db_session.add(inst)
    db_session.commit()

    report = PilotReadinessService.audit_institution_pilot_readiness(db_session, "inst_incomplete")

    assert report["is_pilot_ready"] is False
    assert report["readiness_score_percentage"] < 50
    assert len(report["blocking_items"]) > 0


def test_audit_pilot_readiness_complete_veritas(db_session):
    """Verify that a fully configured institution achieves 100% pilot readiness."""
    # 1. Institution Hierarchy
    inst = Institution(id="veritas_uni", name="Veritas University", code="VUNA")
    fac = Faculty(id="fac_fnas", institution_id="veritas_uni", name="Natural & Applied Sciences", code="FNAS")
    dept = Department(id="dept_cs", faculty_id="fac_fnas", institution_id="veritas_uni", name="Computer Science", code="CSC")
    prog = Program(id="prog_cs", department_id="dept_cs", institution_id="veritas_uni", name="BSc Computer Science", code="CSC")

    # 2. Academic Calendar
    acad_sess = AcademicSession(id="sess_2025_2026", institution_id="veritas_uni", name="2025/2026", is_current=True)
    sem = Semester(id="sem_first", session_id="sess_2025_2026", institution_id="veritas_uni", name="FIRST", is_current=True)

    # 3. Course with Syllabus
    course = Course(
        id="course_csc301",
        institution_id="veritas_uni",
        department_id="dept_cs",
        code="CSC 301",
        title="Data Structures and Algorithms",
        syllabus=[{"topic": "Dynamic Programming", "subtopics": ["Optimal Substructure"]}],
    )

    # 4. Offering, Schedule & Enrollment
    offering = CourseOffering(id="off_csc301", course_id="course_csc301", institution_id="veritas_uni", status="active")
    sched = CourseSchedule(
        id="sched_1",
        course_id="course_csc301",
        institution_id="veritas_uni",
        day_of_week=1,
        start_time=time(9, 0),
        end_time=time(11, 0),
        venue="Lab 2",
    )
    enr = StudentEnrollment(id="enr_1", user_id="student_vuna_1", course_id="course_csc301", status="active")

    db_session.add_all([inst, fac, dept, prog, acad_sess, sem, course, offering, sched, enr])
    db_session.commit()

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


def test_admin_service_audit_and_sis_import(db_session):
    """Verify administrative audit logging and bulk SIS import engine."""
    inst = Institution(id="inst_audit_test", name="Audit Test Uni", code="ATU")
    db_session.add(inst)
    db_session.commit()

    # 1. Test Admin Action Logging
    log_entry = AdminService.log_admin_action(
        db=db_session,
        institution_id="inst_audit_test",
        actor_id="admin_user_01",
        action="UPDATE_SECURITY_POLICY",
        resource_type="institution",
        resource_id="inst_audit_test",
        payload={"ip_filter": "192.168.1.0/24"},
    )
    assert log_entry.id.startswith("audit_")

    logs = AdminService.get_audit_logs(db_session, "inst_audit_test")
    assert len(logs) == 1
    assert logs[0].action == "UPDATE_SECURITY_POLICY"

    # 2. Test Bulk SIS Import (Courses)
    course_records = [
        {
            "code": "CSC 101",
            "title": "Introduction to Computer Science",
            "level": 100,
            "credit_units": 3,
            "syllabus": [{"topic": "Foundations", "subtopics": ["Binary", "Logic"]}],
        },
        {
            "code": "CSC 102",
            "title": "Procedural Programming",
            "level": 100,
            "credit_units": 3,
            "syllabus": [{"topic": "Loops", "subtopics": ["While", "For"]}],
        },
    ]

    import_res = AdminService.bulk_sis_import(
        db=db_session,
        institution_id="inst_audit_test",
        import_type="courses",
        records=course_records,
    )

    assert import_res["status"] == "completed"
    assert import_res["total_records"] == 2
    assert import_res["processed_records"] == 2

    # Verify courses created
    c101 = db_session.query(Course).filter(Course.code == "CSC 101").first()
    assert c101 is not None
    assert c101.title == "Introduction to Computer Science"


def test_cost_intelligence_spend_and_throttling(db_session):
    """Verify departmental cost calculation, automated throttling, and learning ROI."""
    course = Course(
        id="c_cost_test",
        institution_id="inst_cost_uni",
        department_id="dept_cs_cost",
        code="CSC 401",
        title="Distributed Systems",
    )
    db_session.add(course)

    # Seed 5 signals and knowledge state
    ks = StudentKnowledgeState(
        id="ks_cost",
        user_id="std_cost_1",
        course_id="c_cost_test",
        topic="Consensus",
        subtopic="Raft",
        mastery_score=80,
    )
    db_session.add(ks)
    for i in range(5):
        sig = LearningSignal(
            id=f"sig_{i}",
            user_id="std_cost_1",
            course_id="c_cost_test",
            topic="Consensus",
            subtopic="Raft",
            signal_type="quiz_attempt",
            score=1,
            max_score=1,
        )
        db_session.add(sig)
    db_session.commit()

    # Test Nominal state
    report = CostIntelligenceService.get_department_cost_intelligence(
        db=db_session,
        institution_id="inst_cost_uni",
        department_id="dept_cs_cost",
    )
    assert report["throttle_status"] == "NOMINAL"
    assert report["is_automated_throttling_active"] is False
    assert report["learning_roi"]["total_mastery_points_gained"] == 80
    assert report["learning_roi"]["cost_per_mastery_point_usd"] > 0

    # Test Throttled state with custom spend override ($550 > $500 budget ceiling)
    throttled_report = CostIntelligenceService.get_department_cost_intelligence(
        db=db_session,
        institution_id="inst_cost_uni",
        department_id="dept_cs_cost",
        custom_spend_override=550.00,
    )
    assert throttled_report["throttle_status"] == "THROTTLED"
    assert throttled_report["is_automated_throttling_active"] is True

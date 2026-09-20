"""
Unit tests for Cohort Course Offerings, Credit & Audit Enrollments, and Timetable Querying.
"""
from datetime import date, time, datetime
from unittest.mock import MagicMock

from academic_service.models.orm import (
    Course,
    CourseOffering,
    CourseSchedule,
    StudentEnrollment,
    Lecture,
)
from academic_service.services.academic_service import AcademicService
from academic_service.services.timetable_service import TimetableService
from academic_service.models.schema import EnrollmentRequest


def test_cohort_enrollment_credit_and_audit():
    """Verify that cohort enrollment correctly records 'credit' and 'audit' enrollment types."""
    db_mock = MagicMock()

    course = Course(
        id="course_csc301",
        code="CSC 301",
        title="Data Structures and Algorithms",
        institution_id="inst_veritas",
    )
    offering = CourseOffering(
        id="offering_csc301_2026_1",
        course_id="course_csc301",
        institution_id="inst_veritas",
        semester_id="sem_2025_2026_first",
        capacity=150,
        status="active",
    )

    # 1. Credit Enrollment
    def query_credit_effect(model):
        m = MagicMock()
        if model == CourseOffering:
            m.filter.return_value.first.return_value = offering
        elif model == Course:
            m.filter.return_value.first.return_value = course
        elif model == StudentEnrollment:
            m.filter.return_value.first.return_value = None  # New enrollment
        return m

    db_mock.query.side_effect = query_credit_effect

    resp_credit = AcademicService.enroll(
        db=db_mock,
        user_id="student_credit_1",
        course_offering_id="offering_csc301_2026_1",
        enrollment_type="credit",
    )

    assert resp_credit.user_id == "student_credit_1"
    assert resp_credit.course_id == "course_csc301"
    assert resp_credit.course_offering_id == "offering_csc301_2026_1"
    assert resp_credit.enrollment_type == "credit"
    assert resp_credit.status == "active"
    assert resp_credit.course_code == "CSC 301"

    # 2. Audit Enrollment
    def query_audit_effect(model):
        m = MagicMock()
        if model == CourseOffering:
            m.filter.return_value.first.return_value = offering
        elif model == Course:
            m.filter.return_value.first.return_value = course
        elif model == StudentEnrollment:
            m.filter.return_value.first.return_value = None  # New enrollment
        return m

    db_mock.query.side_effect = query_audit_effect

    resp_audit = AcademicService.enroll(
        db=db_mock,
        user_id="student_audit_2",
        course_offering_id="offering_csc301_2026_1",
        enrollment_type="audit",
    )

    assert resp_audit.user_id == "student_audit_2"
    assert resp_audit.course_id == "course_csc301"
    assert resp_audit.course_offering_id == "offering_csc301_2026_1"
    assert resp_audit.enrollment_type == "audit"
    assert resp_audit.status == "active"


def test_timetable_query_for_enrolled_student():
    """Verify that timetable querying returns upcoming lectures and scheduled slots for an enrolled student."""
    db_mock = MagicMock()

    course = Course(
        id="course_csc301",
        code="CSC 301",
        title="Data Structures and Algorithms",
    )
    enrollment = StudentEnrollment(
        id="enr_1",
        user_id="student_1",
        course_id="course_csc301",
        course_offering_id="offering_csc301_2026_1",
        enrollment_type="credit",
        status="active",
    )

    # Explicit lecture on Wednesday 2026-09-23
    lecture = Lecture(
        id="lec_1",
        course_id="course_csc301",
        course_offering_id="offering_csc301_2026_1",
        lecture_number=5,
        title="Dynamic Programming & Memoization",
        date=date(2026, 9, 23),
        start_time=time(10, 0, 0),
        end_time=time(12, 0, 0),
        topics_covered=["Dynamic Programming", "Memoization"],
        is_processed=True,
    )

    # Recurring schedule on Friday (day 5)
    schedule = CourseSchedule(
        id="sched_1",
        course_id="course_csc301",
        institution_id="inst_veritas",
        day_of_week=5,  # Friday
        start_time=time(14, 0, 0),
        end_time=time(16, 0, 0),
        venue="Lab 3",
        recurrence="weekly",
    )

    def query_effect(model):
        m = MagicMock()
        if model == StudentEnrollment:
            m.filter.return_value.all.return_value = [enrollment]
        elif model == Course:
            m.filter.return_value.all.return_value = [course]
        elif model == Lecture:
            m.filter.return_value.all.return_value = [lecture]
        elif model == CourseSchedule:
            m.filter.return_value.all.return_value = [schedule]
        return m

    db_mock.query.side_effect = query_effect

    # Window: Monday 2026-09-21 to Sunday 2026-09-27
    start_d = date(2026, 9, 21)
    end_d = date(2026, 9, 27)

    sessions = TimetableService.get_upcoming_lecture_sessions(
        db=db_mock,
        user_id="student_1",
        start_date=start_d,
        end_date=end_d,
    )

    assert len(sessions) == 2

    # First session: explicit lecture on Wednesday 2026-09-23
    assert sessions[0]["source"] == "lecture"
    assert sessions[0]["date"] == "2026-09-23"
    assert sessions[0]["start_time"] == "10:00:00"
    assert sessions[0]["course_code"] == "CSC 301"
    assert sessions[0]["enrollment_type"] == "credit"
    assert sessions[0]["title"] == "Dynamic Programming & Memoization"

    # Second session: recurring schedule on Friday 2026-09-25
    assert sessions[1]["source"] == "schedule"
    assert sessions[1]["date"] == "2026-09-25"
    assert sessions[1]["start_time"] == "14:00:00"
    assert sessions[1]["venue"] == "Lab 3"
    assert sessions[1]["enrollment_type"] == "credit"


def test_timetable_query_by_course_offering():
    """Verify that timetable querying by course offering retrieves lectures for that cohort."""
    db_mock = MagicMock()

    course = Course(
        id="course_csc301",
        code="CSC 301",
        title="Data Structures and Algorithms",
    )
    offering = CourseOffering(
        id="offering_csc301_2026_1",
        course_id="course_csc301",
        institution_id="inst_veritas",
        capacity=150,
        status="active",
    )
    lecture = Lecture(
        id="lec_10",
        course_id="course_csc301",
        course_offering_id="offering_csc301_2026_1",
        lecture_number=10,
        title="Graph Algorithms & Shortest Path",
        date=date(2026, 9, 22),
        start_time=time(9, 0, 0),
        end_time=time(11, 0, 0),
        topics_covered=["Dijkstra", "Bellman-Ford"],
        is_processed=False,
    )

    def query_effect(model):
        m = MagicMock()
        if model == CourseOffering:
            m.filter.return_value.first.return_value = offering
        elif model == Course:
            m.filter.return_value.all.return_value = [course]
        elif model == Lecture:
            m.filter.return_value.all.return_value = [lecture]
        elif model == CourseSchedule:
            m.filter.return_value.all.return_value = []
        return m

    db_mock.query.side_effect = query_effect

    sessions = TimetableService.get_upcoming_lecture_sessions(
        db=db_mock,
        course_offering_id="offering_csc301_2026_1",
        start_date=date(2026, 9, 21),
        end_date=date(2026, 9, 27),
    )

    assert len(sessions) == 1
    assert sessions[0]["id"] == "lec_10"
    assert sessions[0]["course_offering_id"] == "offering_csc301_2026_1"
    assert sessions[0]["title"] == "Graph Algorithms & Shortest Path"


def test_timetable_query_empty_when_no_enrollments():
    """Verify that querying timetable for a student with no enrollments returns empty list."""
    db_mock = MagicMock()
    db_mock.query(StudentEnrollment).filter.return_value.all.return_value = []

    sessions = TimetableService.get_upcoming_lecture_sessions(
        db=db_mock,
        user_id="student_no_courses",
        start_date=date(2026, 9, 21),
        end_date=date(2026, 9, 27),
    )

    assert sessions == []


def test_course_offering_model_defaults():
    """Verify CourseOffering ORM entity default values and initialization."""
    offering = CourseOffering(
        id="off_test_1",
        course_id="course_1",
        institution_id="inst_1",
    )
    assert offering.capacity == 150
    assert offering.status == "active"

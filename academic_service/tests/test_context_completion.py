"""
Unit tests for Context Completion Engine.
"""
from unittest.mock import MagicMock
from academic_service.services.context_completion import ContextCompletionEngine
from academic_service.models.orm import Course, CourseSchedule, StudentEnrollment, ContextGap


def test_context_gap_detection_missing_schedule():
    """Verify that a course without any timetable schedule generates a gap item."""
    db_mock = MagicMock()

    enrollment = StudentEnrollment(user_id="usr_1", course_id="course_csc301")
    course = Course(id="course_csc301", code="CSC 301", title="Data Structures")

    # Mock DB returns enrollment, course, but empty schedules list
    db_mock.query(StudentEnrollment).filter().all.return_value = [enrollment]
    db_mock.query(Course).filter().first.return_value = course
    db_mock.query(CourseSchedule).filter().all.return_value = [] # No schedule

    # No existing gap in DB
    db_mock.query(ContextGap).filter().first.return_value = None

    # Expected created gap
    created_gap = ContextGap(
        id="gap_1",
        user_id="usr_1",
        course_id="course_csc301",
        gap_type="missing_class_time",
        prompt_question="I have your CSC 301 (Data Structures) class, but I don't know when it holds.",
        field_target="course_schedules",
        is_resolved=False,
    )
    db_mock.query(ContextGap).filter().all.return_value = [created_gap]

    gaps = ContextCompletionEngine.scan_and_generate_gaps(db=db_mock, user_id="usr_1")

    assert len(gaps) == 1
    assert gaps[0].gap_type == "missing_class_time"
    assert "CSC 301" in gaps[0].prompt_question

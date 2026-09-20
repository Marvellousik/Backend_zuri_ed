"""
Unit tests for Timetable Service and Class Countdown arithmetic.
"""
from datetime import datetime, time
from unittest.mock import MagicMock
from academic_service.services.timetable_service import TimetableService
from academic_service.models.orm import CourseSchedule, Course, StudentEnrollment


def test_next_class_calculation_same_day():
    """Verify that a class later on the same day is correctly identified."""
    db_mock = MagicMock()

    # Mock user enrollment in CSC 301
    enrollment = StudentEnrollment(user_id="usr_1", course_id="course_csc301")
    db_mock.query().filter().all.side_effect = [
        [enrollment],  # enrollments query
    ]

    # Course and schedule (Monday 09:00:00)
    course = Course(id="course_csc301", code="CSC 301", title="Data Structures")
    schedule = CourseSchedule(
        id="s1",
        course_id="course_csc301",
        day_of_week=1, # Monday
        start_time=time(9, 0, 0),
        end_time=time(11, 0, 0),
        venue="Science Lab 2",
    )

    db_mock.query().join().filter().all.return_value = [(schedule, course)]

    # Test time: Monday 08:30:00 (30 minutes before class)
    simulated_now = datetime(2026, 8, 31, 8, 30, 0) # 2026-08-31 is Monday (isoweekday 1)

    result = TimetableService.get_next_class_for_student(
        db=db_mock,
        user_id="usr_1",
        current_time=simulated_now,
    )

    assert result.has_upcoming is True
    assert result.course_code == "CSC 301"
    assert result.minutes_until_class == 30
    assert result.venue == "Science Lab 2"

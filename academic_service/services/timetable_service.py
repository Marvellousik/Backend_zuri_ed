"""
Timetable and Schedule Service for Lexi.
Builds student weekly timetables, calculates next upcoming classes, and computes minute countdowns.
"""
import uuid
import logging
from datetime import datetime, timezone, time, date, timedelta
from typing import List, Optional, Dict, Any, Union
from sqlalchemy.orm import Session

from academic_service.models.orm import CourseSchedule, Course, StudentEnrollment, CourseOffering, Lecture
from academic_service.models.schema import CourseScheduleDTO, NextClassResponse, TimetableSlotCreate, LectureSessionResponse

logger = logging.getLogger(__name__)

DAY_NAMES = {1: "Monday", 2: "Tuesday", 3: "Wednesday", 4: "Thursday", 5: "Friday", 6: "Saturday", 7: "Sunday"}


class TimetableService:
    """Calculates student schedules and upcoming class countdowns."""

    @staticmethod
    def get_student_timetable(db: Session, user_id: str) -> List[Dict[str, Any]]:
        """Fetch full weekly timetable grouped by day for a student."""
        enrollments = (
            db.query(StudentEnrollment)
            .filter(StudentEnrollment.user_id == user_id, StudentEnrollment.status == "active")
            .all()
        )
        course_ids = [e.course_id for e in enrollments]
        if not course_ids:
            return []

        schedules = (
            db.query(CourseSchedule, Course)
            .join(Course, CourseSchedule.course_id == Course.id)
            .filter(CourseSchedule.course_id.in_(course_ids))
            .order_by(CourseSchedule.day_of_week, CourseSchedule.start_time)
            .all()
        )

        result = []
        for schedule, course in schedules:
            result.append({
                "id": schedule.id,
                "course_id": course.id,
                "course_code": course.code,
                "course_title": course.title,
                "day_of_week": schedule.day_of_week,
                "day_name": DAY_NAMES.get(schedule.day_of_week, ""),
                "start_time": schedule.start_time.strftime("%H:%M:%S") if hasattr(schedule.start_time, "strftime") else str(schedule.start_time),
                "end_time": schedule.end_time.strftime("%H:%M:%S") if hasattr(schedule.end_time, "strftime") else str(schedule.end_time),
                "venue": schedule.venue,
            })
        return result

    @staticmethod
    def get_next_class_for_student(
        db: Session,
        user_id: str,
        current_time: Optional[datetime] = None
    ) -> NextClassResponse:
        """
        Calculate the immediate next upcoming class for the student and minutes remaining.
        Supports wrap-around across weekdays and weekends.
        """
        now = current_time or datetime.now()
        current_weekday = now.isoweekday()  # 1 = Monday, 7 = Sunday
        current_time_val = now.time()

        enrollments = (
            db.query(StudentEnrollment)
            .filter(StudentEnrollment.user_id == user_id, StudentEnrollment.status == "active")
            .all()
        )
        course_ids = [e.course_id for e in enrollments]
        if not course_ids:
            return NextClassResponse(has_upcoming=False, message="No active courses enrolled")

        schedules = (
            db.query(CourseSchedule, Course)
            .join(Course, CourseSchedule.course_id == Course.id)
            .filter(CourseSchedule.course_id.in_(course_ids))
            .all()
        )

        if not schedules:
            return NextClassResponse(has_upcoming=False, message="No timetable schedules found for enrolled courses")

        # Find the earliest class occurrence ahead of `now`
        candidate_classes = []
        for schedule, course in schedules:
            slot_day = schedule.day_of_week
            slot_start = schedule.start_time

            # Days difference (0 to 6 days ahead)
            day_diff = (slot_day - current_weekday) % 7
            
            # If slot is today but already passed, push to next week (7 days ahead)
            if day_diff == 0 and slot_start <= current_time_val:
                day_diff = 7

            target_date = now.date() + timedelta(days=day_diff)
            target_dt = datetime.combine(target_date, slot_start)

            total_minutes = int((target_dt - now).total_seconds() // 60)
            candidate_classes.append((total_minutes, schedule, course))

        # Sort by earliest upcoming
        candidate_classes.sort(key=lambda x: x[0])
        minutes_remaining, next_sched, next_course = candidate_classes[0]

        return NextClassResponse(
            has_upcoming=True,
            course_code=next_course.code,
            course_title=next_course.title,
            day_of_week=next_sched.day_of_week,
            start_time=next_sched.start_time.strftime("%H:%M:%S") if hasattr(next_sched.start_time, "strftime") else str(next_sched.start_time),
            venue=next_sched.venue or "Main Lecture Hall",
            minutes_until_class=minutes_remaining,
            message=f"{next_course.code} starts in {minutes_remaining} minutes at {next_sched.venue or 'lecture hall'}",
        )

    @staticmethod
    def add_timetable_slot(db: Session, slot_data: TimetableSlotCreate, institution_id: str = "veritas_uni") -> CourseScheduleDTO:
        """Manually add or update a timetable slot for a course."""
        # Parse time strings
        start_parts = [int(p) for p in slot_data.start_time.split(":")]
        end_parts = [int(p) for p in slot_data.end_time.split(":")]
        
        start_t = time(hour=start_parts[0], minute=start_parts[1], second=start_parts[2] if len(start_parts) > 2 else 0)
        end_t = time(hour=end_parts[0], minute=end_parts[1], second=end_parts[2] if len(end_parts) > 2 else 0)

        slot = CourseSchedule(
            id=str(uuid.uuid4()),
            course_id=slot_data.course_id,
            institution_id=institution_id,
            day_of_week=slot_data.day_of_week,
            start_time=start_t,
            end_time=end_t,
            venue=slot_data.venue,
            recurrence="weekly",
        )
        db.add(slot)
        db.commit()

        return CourseScheduleDTO(
            id=slot.id,
            course_id=slot.course_id,
            day_of_week=slot.day_of_week,
            day_name=DAY_NAMES.get(slot.day_of_week, ""),
            start_time=slot_data.start_time,
            end_time=slot_data.end_time,
            venue=slot.venue,
            recurrence=slot.recurrence,
        )

    @staticmethod
    def get_upcoming_lecture_sessions(
        db: Session,
        user_id: Optional[str] = None,
        course_offering_id: Optional[str] = None,
        start_date: Optional[Union[date, str]] = None,
        end_date: Optional[Union[date, str]] = None,
    ) -> List[Dict[str, Any]]:
        """
        Query upcoming lecture sessions by course offering or user enrollment for a given date window.
        Combines canonical lecture sessions and recurring timetable schedules.
        """
        # Resolve date window
        if start_date is None:
            start_d = date.today()
        elif isinstance(start_date, str):
            start_d = date.fromisoformat(start_date)
        elif isinstance(start_date, datetime):
            start_d = start_date.date()
        else:
            start_d = start_date

        if end_date is None:
            end_d = start_d + timedelta(days=7)
        elif isinstance(end_date, str):
            end_d = date.fromisoformat(end_date)
        elif isinstance(end_date, datetime):
            end_d = end_date.date()
        else:
            end_d = end_date

        course_ids = []
        offering_map: Dict[str, Optional[str]] = {}
        enrollment_type_map: Dict[str, str] = {}

        if course_offering_id:
            offering = db.query(CourseOffering).filter(CourseOffering.id == course_offering_id).first()
            if offering:
                course_ids = [offering.course_id]
                offering_map[offering.course_id] = offering.id
            else:
                # Check if course_offering_id matches a course.id or code directly
                course = (
                    db.query(Course)
                    .filter((Course.id == course_offering_id) | (Course.code.ilike(course_offering_id.strip())))
                    .first()
                )
                if course:
                    course_ids = [course.id]
                    offering_map[course.id] = None
                else:
                    return []
        elif user_id:
            enrollments = (
                db.query(StudentEnrollment)
                .filter(StudentEnrollment.user_id == user_id, StudentEnrollment.status == "active")
                .all()
            )
            if not enrollments:
                return []
            for e in enrollments:
                course_ids.append(e.course_id)
                if e.course_offering_id:
                    offering_map[e.course_id] = e.course_offering_id
                enrollment_type_map[e.course_id] = getattr(e, "enrollment_type", "credit") or "credit"
        else:
            return []

        if not course_ids:
            return []

        courses = db.query(Course).filter(Course.id.in_(course_ids)).all()
        courses_by_id = {c.id: c for c in courses}

        sessions: List[Dict[str, Any]] = []
        covered_dates = set()

        # 1. Fetch canonical lectures scheduled in this date window
        lectures = (
            db.query(Lecture)
            .filter(
                Lecture.course_id.in_(course_ids),
                Lecture.date >= start_d,
                Lecture.date <= end_d,
            )
            .all()
        )

        for l in lectures:
            l_offering_id = getattr(l, "course_offering_id", None)
            if course_offering_id and l_offering_id and l_offering_id != course_offering_id:
                continue

            course = courses_by_id.get(l.course_id)
            c_code = course.code if course else ""
            c_title = course.title if course else ""
            off_id = l_offering_id or offering_map.get(l.course_id)
            enr_type = enrollment_type_map.get(l.course_id)

            l_date_str = l.date.isoformat() if hasattr(l.date, "isoformat") else str(l.date)
            covered_dates.add((l.course_id, l.date))

            sessions.append({
                "id": l.id,
                "course_id": l.course_id,
                "course_code": c_code,
                "course_title": c_title,
                "course_offering_id": off_id,
                "enrollment_type": enr_type,
                "lecture_number": l.lecture_number,
                "title": l.title or f"{c_code} Lecture {l.lecture_number}",
                "date": l_date_str,
                "start_time": l.start_time.strftime("%H:%M:%S") if hasattr(l.start_time, "strftime") else str(l.start_time) if l.start_time else "09:00:00",
                "end_time": l.end_time.strftime("%H:%M:%S") if hasattr(l.end_time, "strftime") else str(l.end_time) if l.end_time else "11:00:00",
                "venue": getattr(l, "venue", None) or "Main Lecture Hall",
                "topics_covered": l.topics_covered or [],
                "is_processed": getattr(l, "is_processed", False),
                "source": "lecture",
            })

        # 2. Project recurring course schedules into the date window
        schedules = db.query(CourseSchedule).filter(CourseSchedule.course_id.in_(course_ids)).all()
        num_days = (end_d - start_d).days + 1

        for day_offset in range(num_days):
            curr_date = start_d + timedelta(days=day_offset)
            curr_weekday = curr_date.isoweekday()  # 1 = Monday, 7 = Sunday

            for s in schedules:
                if s.day_of_week == curr_weekday:
                    if (s.course_id, curr_date) in covered_dates:
                        continue

                    course = courses_by_id.get(s.course_id)
                    c_code = course.code if course else ""
                    c_title = course.title if course else ""
                    off_id = offering_map.get(s.course_id)
                    enr_type = enrollment_type_map.get(s.course_id)

                    sessions.append({
                        "id": f"sched_{s.id}_{curr_date.isoformat()}",
                        "course_id": s.course_id,
                        "course_code": c_code,
                        "course_title": c_title,
                        "course_offering_id": off_id,
                        "enrollment_type": enr_type,
                        "lecture_number": None,
                        "title": f"{c_code} Scheduled Lecture",
                        "date": curr_date.isoformat(),
                        "start_time": s.start_time.strftime("%H:%M:%S") if hasattr(s.start_time, "strftime") else str(s.start_time),
                        "end_time": s.end_time.strftime("%H:%M:%S") if hasattr(s.end_time, "strftime") else str(s.end_time),
                        "venue": s.venue or "Main Lecture Hall",
                        "topics_covered": [],
                        "is_processed": False,
                        "source": "schedule",
                    })

        # Sort chronologically by date and start_time
        sessions.sort(key=lambda x: (x.get("date") or "", x.get("start_time") or ""))
        return sessions

    @staticmethod
    def get_upcoming_lecture_session_dtos(
        db: Session,
        user_id: Optional[str] = None,
        course_offering_id: Optional[str] = None,
        start_date: Optional[Union[date, str]] = None,
        end_date: Optional[Union[date, str]] = None,
    ) -> List[LectureSessionResponse]:
        """Fetch upcoming lecture sessions as validated Pydantic response models."""
        raw_sessions = TimetableService.get_upcoming_lecture_sessions(
            db=db,
            user_id=user_id,
            course_offering_id=course_offering_id,
            start_date=start_date,
            end_date=end_date,
        )
        return [LectureSessionResponse(**s) for s in raw_sessions]

    @staticmethod
    def get_upcoming_sessions_by_offering(
        db: Session,
        course_offering_id: str,
        start_date: Optional[Union[date, str]] = None,
        end_date: Optional[Union[date, str]] = None,
    ) -> List[Dict[str, Any]]:
        """Convenience method for offering-scoped timetable lookups."""
        return TimetableService.get_upcoming_lecture_sessions(
            db=db,
            course_offering_id=course_offering_id,
            start_date=start_date,
            end_date=end_date,
        )

    @staticmethod
    def get_upcoming_sessions_by_user(
        db: Session,
        user_id: str,
        start_date: Optional[Union[date, str]] = None,
        end_date: Optional[Union[date, str]] = None,
    ) -> List[Dict[str, Any]]:
        """Convenience method for student enrollment-scoped timetable lookups."""
        return TimetableService.get_upcoming_lecture_sessions(
            db=db,
            user_id=user_id,
            start_date=start_date,
            end_date=end_date,
        )

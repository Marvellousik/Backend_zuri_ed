"""
Academic Structure and Course Service for Lexi.
Manages institutions, departments, courses, syllabi, canonical lectures, and student enrollments.
"""
import uuid
import logging
from typing import List, Optional, Dict, Any
from sqlalchemy.orm import Session

from academic_service.models.orm import (
    Institution,
    Faculty,
    Department,
    Course,
    CourseSchedule,
    StudentEnrollment,
    Lecture,
    AcademicEvent,
    Program,
    AcademicSession,
    Semester,
    CourseOffering,
)
from academic_service.models.schema import (
    CourseDetailDTO,
    CourseScheduleDTO,
    LectureDTO,
    AcademicEventDTO,
    ProgramResponse,
    DepartmentHierarchyResponse,
    FacultyHierarchyResponse,
    InstitutionHierarchyResponse,
    EnrollmentRequest,
    EnrollmentResponse,
    CourseOfferingResponse,
)

logger = logging.getLogger(__name__)

DAY_NAMES = {1: "Monday", 2: "Tuesday", 3: "Wednesday", 4: "Thursday", 5: "Friday", 6: "Saturday", 7: "Sunday"}


class AcademicService:
    """Core domain logic for courses, syllabus, and enrollments."""

    @staticmethod
    def get_enrolled_courses(db: Session, user_id: str) -> List[CourseDetailDTO]:
        """Fetch all courses a student is actively enrolled in."""
        enrollments = (
            db.query(StudentEnrollment)
            .filter(StudentEnrollment.user_id == user_id, StudentEnrollment.status == "active")
            .all()
        )
        course_ids = [e.course_id for e in enrollments]
        if not course_ids:
            return []

        courses = db.query(Course).filter(Course.id.in_(course_ids)).all()
        return [AcademicService._to_course_dto(c) for c in courses]

    @staticmethod
    def get_course_by_id(db: Session, course_id: str) -> Optional[CourseDetailDTO]:
        """Fetch full details for a course including syllabus, schedules, and lectures."""
        # Query by ID or by Course Code (case-insensitive)
        course = (
            db.query(Course)
            .filter((Course.id == course_id) | (Course.code.ilike(course_id.strip())))
            .first()
        )
        if not course:
            return None
        return AcademicService._to_course_dto(course)

    @staticmethod
    def enroll_student(
        db: Session,
        user_id: str,
        course_identifier: str,
        semester: str = "2025/2026_FIRST",
        course_offering_id: Optional[str] = None,
        enrollment_type: str = "credit",
    ) -> CourseDetailDTO:
        """Enroll a student into a course by ID or Code with optional offering and enrollment type."""
        course = (
            db.query(Course)
            .filter((Course.id == course_identifier) | (Course.code.ilike(course_identifier.strip())))
            .first()
        )
        if not course:
            raise ValueError(f"Course '{course_identifier}' not found in academic catalog.")

        # Check existing enrollment
        existing = (
            db.query(StudentEnrollment)
            .filter(StudentEnrollment.user_id == user_id, StudentEnrollment.course_id == course.id)
            .first()
        )
        if not existing:
            enrollment = StudentEnrollment(
                id=str(uuid.uuid4()),
                user_id=user_id,
                course_id=course.id,
                course_offering_id=course_offering_id,
                enrollment_type=enrollment_type,
                semester=semester,
                status="active",
            )
            db.add(enrollment)
            db.commit()
            logger.info(f"Enrolled user {user_id} into course {course.code} (offering: {course_offering_id}, type: {enrollment_type})")
        else:
            if course_offering_id and not existing.course_offering_id:
                existing.course_offering_id = course_offering_id
            if enrollment_type:
                existing.enrollment_type = enrollment_type
            db.commit()

        return AcademicService._to_course_dto(course)

    @staticmethod
    def enroll(
        db: Session,
        user_id: str,
        course_identifier: Optional[str] = None,
        course_offering_id: Optional[str] = None,
        enrollment_type: str = "credit",
        semester: str = "2025/2026_FIRST",
    ) -> EnrollmentResponse:
        """Enroll student into a course offering or course cohort, supporting credit and audit types."""
        course = None
        offering = None

        if course_offering_id:
            offering = db.query(CourseOffering).filter(CourseOffering.id == course_offering_id).first()
            if offering:
                course = db.query(Course).filter(Course.id == offering.course_id).first()
            elif not course_identifier:
                course = (
                    db.query(Course)
                    .filter((Course.id == course_offering_id) | (Course.code.ilike(course_offering_id.strip())))
                    .first()
                )

        if not course and course_identifier:
            course = (
                db.query(Course)
                .filter((Course.id == course_identifier) | (Course.code.ilike(course_identifier.strip())))
                .first()
            )
            if not course:
                offering = db.query(CourseOffering).filter(CourseOffering.id == course_identifier).first()
                if offering:
                    course = db.query(Course).filter(Course.id == offering.course_id).first()

        if not course:
            raise ValueError(f"Course or offering '{course_offering_id or course_identifier}' not found in academic catalog.")

        resolved_offering_id = offering.id if offering else course_offering_id

        # Check existing enrollment
        existing = (
            db.query(StudentEnrollment)
            .filter(StudentEnrollment.user_id == user_id, StudentEnrollment.course_id == course.id)
            .first()
        )

        if existing:
            if resolved_offering_id and not existing.course_offering_id:
                existing.course_offering_id = resolved_offering_id
            if enrollment_type:
                existing.enrollment_type = enrollment_type
            db.commit()
            enrollment = existing
        else:
            enrollment = StudentEnrollment(
                id=str(uuid.uuid4()),
                user_id=user_id,
                course_id=course.id,
                course_offering_id=resolved_offering_id,
                enrollment_type=enrollment_type,
                semester=semester,
                status="active",
            )
            db.add(enrollment)
            db.commit()
            logger.info(f"Enrolled user {user_id} into course {course.code} (offering: {resolved_offering_id}, type: {enrollment_type})")

        return EnrollmentResponse(
            id=enrollment.id,
            user_id=enrollment.user_id,
            course_id=course.id,
            course_offering_id=enrollment.course_offering_id,
            enrollment_type=enrollment.enrollment_type or "credit",
            grade=enrollment.grade,
            semester=enrollment.semester,
            status=enrollment.status,
            course_code=course.code,
            course_title=course.title,
            created_at=enrollment.created_at.isoformat() if hasattr(enrollment.created_at, "isoformat") else str(enrollment.created_at),
        )

    @staticmethod
    def get_course_offerings(db: Session, course_id: Optional[str] = None, institution_id: Optional[str] = None) -> List[CourseOfferingResponse]:
        """Fetch active course offerings optionally filtered by course or institution."""
        query = db.query(CourseOffering)
        if course_id:
            query = query.filter(CourseOffering.course_id == course_id)
        if institution_id:
            query = query.filter(CourseOffering.institution_id == institution_id)
        offerings = query.all()
        return [
            CourseOfferingResponse(
                id=o.id,
                course_id=o.course_id,
                institution_id=o.institution_id,
                semester_id=o.semester_id,
                lecturer_id=o.lecturer_id,
                capacity=o.capacity or 150,
                status=o.status or "active",
                created_at=o.created_at.isoformat() if hasattr(o.created_at, "isoformat") else str(o.created_at),
                course_code=o.course.code if getattr(o, "course", None) else None,
                course_title=o.course.title if getattr(o, "course", None) else None,
            )
            for o in offerings
        ]

    @staticmethod
    def _to_course_dto(c: Course) -> CourseDetailDTO:
        """Convert Course ORM to detailed DTO."""
        schedules_dto = [
            CourseScheduleDTO(
                id=s.id,
                course_id=s.course_id,
                day_of_week=s.day_of_week,
                day_name=DAY_NAMES.get(s.day_of_week, f"Day {s.day_of_week}"),
                start_time=s.start_time.strftime("%H:%M:%S") if hasattr(s.start_time, "strftime") else str(s.start_time),
                end_time=s.end_time.strftime("%H:%M:%S") if hasattr(s.end_time, "strftime") else str(s.end_time),
                venue=s.venue,
                recurrence=s.recurrence or "weekly",
            )
            for s in (c.schedules or [])
        ]

        lectures_dto = [
            LectureDTO(
                id=l.id,
                course_id=l.course_id,
                lecture_number=l.lecture_number,
                title=l.title,
                date=l.date.isoformat() if l.date else None,
                start_time=str(l.start_time) if l.start_time else None,
                end_time=str(l.end_time) if l.end_time else None,
                topics_covered=l.topics_covered or [],
                slides_url=l.slides_url,
                summary_text=l.summary_text,
                is_processed=l.is_processed,
            )
            for l in (c.lectures or [])
        ]

        events_dto = [
            AcademicEventDTO(
                id=e.id,
                course_id=e.course_id,
                event_type=e.event_type,
                title=e.title,
                description=e.description,
                due_date=e.due_date.isoformat() if e.due_date else "",
                weight_percent=e.weight_percent,
            )
            for e in (c.events or [])
        ]

        return CourseDetailDTO(
            id=c.id,
            institution_id=c.institution_id,
            department_id=c.department_id,
            code=c.code,
            title=c.title,
            level=c.level,
            credit_units=c.credit_units,
            syllabus=c.syllabus or [],
            description=c.description,
            schedules=schedules_dto,
            lectures=lectures_dto,
            events=events_dto,
        )

    @staticmethod
    def get_institution_hierarchy(
        institution_id: Any,
        db: Any = None,
    ) -> Optional[InstitutionHierarchyResponse]:
        """
        Fetch full academic hierarchy (faculties, departments, and programs) for an institution.
        Supports both (institution_id, db) and (db, institution_id) invocation conventions.
        """
        if hasattr(institution_id, "query"):
            # Swapped arguments: (db, institution_id)
            db_session = institution_id
            inst_id = str(db) if db is not None else ""
        else:
            db_session = db
            inst_id = str(institution_id) if institution_id is not None else ""

        if not db_session or not inst_id:
            return None

        institution = (
            db_session.query(Institution)
            .filter((Institution.id == inst_id) | (Institution.code.ilike(inst_id.strip())))
            .first()
        )
        if not institution:
            return None

        faculties = db_session.query(Faculty).filter(Faculty.institution_id == institution.id).all()
        if not faculties and getattr(institution, "faculties", None):
            faculties = institution.faculties or []

        fac_list: List[FacultyHierarchyResponse] = []
        all_depts_list: List[DepartmentHierarchyResponse] = []
        all_progs_list: List[ProgramResponse] = []

        for fac in faculties:
            depts = db_session.query(Department).filter(Department.faculty_id == fac.id).all()
            if not depts and getattr(fac, "departments", None):
                depts = fac.departments or []

            dept_responses: List[DepartmentHierarchyResponse] = []
            for dept in depts:
                progs = db_session.query(Program).filter(Program.department_id == dept.id).all()
                if not progs and getattr(dept, "programs", None):
                    progs = dept.programs or []

                prog_responses = [AcademicService._to_program_dto(p) for p in progs]
                all_progs_list.extend(prog_responses)

                dept_dto = DepartmentHierarchyResponse(
                    id=dept.id,
                    faculty_id=dept.faculty_id,
                    institution_id=dept.institution_id,
                    name=dept.name,
                    code=dept.code,
                    programs=prog_responses,
                )
                dept_responses.append(dept_dto)
                all_depts_list.append(dept_dto)

            fac_dto = FacultyHierarchyResponse(
                id=fac.id,
                institution_id=fac.institution_id,
                name=fac.name,
                code=fac.code,
                departments=dept_responses,
            )
            fac_list.append(fac_dto)

        # Check if there are any top-level or direct institution programs
        inst_progs = db_session.query(Program).filter(Program.institution_id == institution.id).all()
        if not inst_progs and getattr(institution, "programs", None):
            inst_progs = institution.programs or []

        seen_prog_ids = {p.id for p in all_progs_list}
        for p in inst_progs:
            if p.id not in seen_prog_ids:
                all_progs_list.append(AcademicService._to_program_dto(p))
                seen_prog_ids.add(p.id)

        return InstitutionHierarchyResponse(
            id=institution.id,
            name=institution.name,
            code=institution.code,
            domain=institution.domain,
            settings=institution.settings,
            faculties=fac_list,
            departments=all_depts_list,
            programs=all_progs_list,
        )

    @staticmethod
    def _to_program_dto(p: Program) -> ProgramResponse:
        """Convert Program ORM to ProgramResponse DTO."""
        created_at = None
        if hasattr(p, "created_at") and p.created_at:
            if hasattr(p.created_at, "isoformat"):
                created_at = p.created_at.isoformat()
            else:
                created_at = str(p.created_at)

        return ProgramResponse(
            id=p.id,
            department_id=p.department_id,
            institution_id=p.institution_id,
            name=p.name,
            code=p.code,
            degree_type=getattr(p, "degree_type", "BSc") or "BSc",
            duration_years=getattr(p, "duration_years", 4) or 4,
            created_at=created_at,
        )


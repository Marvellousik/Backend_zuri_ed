"""
Institutional Pilot Readiness Audit Service for Lexi (Phase 39).
Audits institutional context graph readiness for pilot deployment:
- Hierarchy verification (Faculties, Departments, Programs).
- Academic Calendar (Active Academic Session and Current Semester).
- Course Offerings and Syllabi definitions.
- Timetable weekly schedule slots.
- Enrolled Student Cohorts.
Computes comprehensive readiness score (0-100%) and identifies any blocking items.
"""
import logging
from datetime import datetime, timezone
from typing import Dict, Any, List
from sqlalchemy.orm import Session

from academic_service.models.orm import (
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
)

logger = logging.getLogger(__name__)


class PilotReadinessService:
    """
    Audits an institution's digital readiness to conduct live AI academic pilots.
    """

    @classmethod
    def audit_institution_pilot_readiness(
        cls,
        db: Session,
        institution_id: str,
    ) -> Dict[str, Any]:
        """
        Executes an exhaustive audit across the institution's academic graph.
        Returns readiness score (0-100%), status checklist, and any blocking items.
        """
        inst = db.query(Institution).filter(Institution.id == institution_id).first()
        if not inst:
            return {
                "institution_id": institution_id,
                "readiness_score_percentage": 0,
                "is_pilot_ready": False,
                "checklist": {},
                "blocking_items": [f"Institution record '{institution_id}' does not exist in academic graph."],
                "audited_at": datetime.now(timezone.utc).isoformat(),
            }

        checklist: Dict[str, Dict[str, Any]] = {}
        blocking_items: List[str] = []
        score = 0

        # 1. Hierarchy Check (Faculties, Departments, Programs) - 20 pts
        faculties_count = db.query(Faculty).filter(Faculty.institution_id == institution_id).count()
        depts_count = db.query(Department).filter(Department.institution_id == institution_id).count()
        progs_count = db.query(Program).filter(Program.institution_id == institution_id).count()

        has_hierarchy = faculties_count > 0 and depts_count > 0 and progs_count > 0
        if has_hierarchy:
            score += 20
            checklist["hierarchy"] = {
                "status": "PASSED",
                "score": 20,
                "details": f"{faculties_count} faculties, {depts_count} departments, {progs_count} programs.",
            }
        else:
            checklist["hierarchy"] = {
                "status": "FAILED",
                "score": 0,
                "details": f"Incomplete hierarchy ({faculties_count} faculties, {depts_count} depts, {progs_count} programs).",
            }
            blocking_items.append("Academic hierarchy incomplete: Must define at least one faculty, department, and program.")

        # 2. Academic Calendar (Session & Current Semester) - 15 pts
        active_session = db.query(AcademicSession).filter(AcademicSession.institution_id == institution_id, AcademicSession.is_current == True).first()
        if not active_session:
            active_session = db.query(AcademicSession).filter(AcademicSession.institution_id == institution_id).first()

        current_semester = db.query(Semester).filter(Semester.institution_id == institution_id, Semester.is_current == True).first()

        has_calendar = active_session is not None and current_semester is not None
        if has_calendar:
            score += 15
            checklist["calendar"] = {
                "status": "PASSED",
                "score": 15,
                "details": f"Active Session '{active_session.name}', Current Semester '{current_semester.name}'.",
            }
        else:
            checklist["calendar"] = {
                "status": "FAILED",
                "score": 0,
                "details": "Missing active academic session or current semester.",
            }
            blocking_items.append("Academic calendar incomplete: Must designate an active academic session and current semester.")

        # 3. Course Catalog & Syllabi - 20 pts
        courses = db.query(Course).filter(Course.institution_id == institution_id).all()
        courses_with_syllabus = [c for c in courses if c.syllabus and len(c.syllabus) > 0]

        has_syllabi = len(courses) > 0 and len(courses_with_syllabus) == len(courses)
        if has_syllabi:
            score += 20
            checklist["syllabi"] = {
                "status": "PASSED",
                "score": 20,
                "details": f"All {len(courses)} courses have structured syllabi.",
            }
        elif len(courses_with_syllabus) > 0:
            partial_pts = int((len(courses_with_syllabus) / max(1, len(courses))) * 20)
            score += partial_pts
            checklist["syllabi"] = {
                "status": "PARTIAL",
                "score": partial_pts,
                "details": f"{len(courses_with_syllabus)} of {len(courses)} courses have syllabi.",
            }
            blocking_items.append(f"{len(courses) - len(courses_with_syllabus)} courses missing structured syllabus definitions.")
        else:
            checklist["syllabi"] = {
                "status": "FAILED",
                "score": 0,
                "details": "No courses with structured syllabi found.",
            }
            blocking_items.append("No course syllabi found: Courses must define topic modules for AI contextualization.")

        # 4. Active Course Offerings - 15 pts
        offerings = (
            db.query(CourseOffering)
            .filter(CourseOffering.institution_id == institution_id, CourseOffering.status == "active")
            .all()
        )
        if offerings:
            score += 15
            checklist["course_offerings"] = {
                "status": "PASSED",
                "score": 15,
                "details": f"{len(offerings)} active course cohort offerings.",
            }
        else:
            checklist["course_offerings"] = {
                "status": "FAILED",
                "score": 0,
                "details": "No active course offerings found.",
            }
            blocking_items.append("No active course offerings: Cohort sections must be initialized.")

        # 5. Timetable Schedule Slots - 15 pts
        schedules = (
            db.query(CourseSchedule)
            .filter(CourseSchedule.institution_id == institution_id)
            .all()
        )
        if schedules:
            score += 15
            checklist["timetable_slots"] = {
                "status": "PASSED",
                "score": 15,
                "details": f"{len(schedules)} weekly class timetable slots.",
            }
        else:
            checklist["timetable_slots"] = {
                "status": "FAILED",
                "score": 0,
                "details": "No timetable schedule slots defined.",
            }
            blocking_items.append("No class schedules: Weekly timetable slots required for proactive intervention dispatch.")

        # 6. Enrolled Student Cohorts - 15 pts
        course_ids = [c.id for c in courses]
        if course_ids:
            enrollments_count = (
                db.query(StudentEnrollment)
                .filter(StudentEnrollment.course_id.in_(course_ids), StudentEnrollment.status == "active")
                .count()
            )
        else:
            enrollments_count = 0

        if enrollments_count > 0:
            score += 15
            checklist["enrolled_cohorts"] = {
                "status": "PASSED",
                "score": 15,
                "details": f"{enrollments_count} active student enrollments.",
            }
        else:
            checklist["enrolled_cohorts"] = {
                "status": "FAILED",
                "score": 0,
                "details": "Zero active student enrollments.",
            }
            blocking_items.append("No enrolled students: Cohorts must have enrolled student accounts.")

        is_ready = score >= 80 and len(blocking_items) == 0

        logger.info(
            f"📊 Pilot Readiness Audit for {inst.name} [{institution_id}]: "
            f"Score: {score}% | Ready: {is_ready} | Blockers: {len(blocking_items)}"
        )

        return {
            "institution_id": institution_id,
            "institution_name": inst.name,
            "readiness_score_percentage": score,
            "is_pilot_ready": is_ready,
            "checklist": checklist,
            "blocking_items": blocking_items,
            "summary": f"{inst.name} is {'100% READY for live pilot deployment' if is_ready else 'NOT ready: Resolve blocking items'}.",
            "audited_at": datetime.now(timezone.utc).isoformat(),
        }

"""
Veritas University Seed & Bulk Importer for Lexi Academic Context Graph.
Populates standard faculties, Computer Science department, 300L courses, syllabus, and weekly timetable.
"""
import uuid
import logging
from typing import Optional
from datetime import time, date, datetime, timezone
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
    Lecture,
    AcademicEvent,
    StudentEnrollment,
    StudentKnowledgeState,
    LearningSignal,
    LearningGap,
)
from academic_service.storage.database import get_db_session

logger = logging.getLogger(__name__)


def seed_veritas_university(db: Optional[Session] = None):
    """Seed Veritas University baseline academic environment."""
    if db is not None:
        _seed_veritas_university_data(db)
    else:
        with get_db_session() as session:
            _seed_veritas_university_data(session)


def _seed_veritas_university_data(session: Session):
    """Internal seeder logic with active session."""
    # 1. Institution
    inst = session.query(Institution).filter(Institution.id == "veritas_uni").first()
    if not inst:
        inst = Institution(
            id="veritas_uni",
            name="Veritas University",
            code="VUNA",
            domain="veritas.edu.ng",
            settings={
                "current_semester": "2025/2026_FIRST",
                "semester_start": "2026-08-01",
                "semester_end": "2026-12-15",
            },
        )
        session.add(inst)
        session.flush()

    # 2. Faculty
    faculty = session.query(Faculty).filter(Faculty.id == "fnas_veritas").first()
    if not faculty:
        faculty = Faculty(
            id="fnas_veritas",
            institution_id="veritas_uni",
            name="Faculty of Natural and Applied Sciences",
            code="FNAS",
        )
        session.add(faculty)
        session.flush()

    # 3. Department
    dept = session.query(Department).filter(Department.id == "dept_cs_veritas").first()
    if not dept:
        dept = Department(
            id="dept_cs_veritas",
            faculty_id="fnas_veritas",
            institution_id="veritas_uni",
            name="Department of Computer Science",
            code="CSC",
        )
        session.add(dept)
        session.flush()

    # 3b. Program
    prog = session.query(Program).filter(Program.id == "prog_cs_veritas").first()
    if not prog:
        prog = Program(
            id="prog_cs_veritas",
            department_id="dept_cs_veritas",
            institution_id="veritas_uni",
            name="BSc Computer Science",
            code="CSC",
            degree_type="BSc",
            duration_years=4,
        )
        session.add(prog)
        session.flush()

    # 3c. Academic Session & Semester
    session_obj = session.query(AcademicSession).filter(AcademicSession.id == "sess_2025_2026_veritas").first()
    if not session_obj:
        session_obj = AcademicSession(
            id="sess_2025_2026_veritas",
            institution_id="veritas_uni",
            name="2025/2026",
            start_date=date(2026, 8, 1),
            end_date=date(2027, 7, 31),
            is_current=True,
        )
        session.add(session_obj)
        session.flush()

    semester_obj = session.query(Semester).filter(Semester.id == "sem_2025_2026_first_veritas").first()
    if not semester_obj:
        semester_obj = Semester(
            id="sem_2025_2026_first_veritas",
            session_id="sess_2025_2026_veritas",
            institution_id="veritas_uni",
            name="FIRST",
            start_date=date(2026, 8, 1),
            end_date=date(2026, 12, 15),
            is_current=True,
        )
        session.add(semester_obj)
        session.flush()

        # 4. Courses (CSC 301, CSC 305, MTH 302)
        courses_data = [
            {
                "id": "course_csc301_veritas",
                "code": "CSC 301",
                "title": "Data Structures & Algorithms",
                "level": 300,
                "credit_units": 3,
                "syllabus": [
                    {"topic": "Recursion & Divide and Conquer", "subtopics": ["Binary Search", "Merge Sort", "Quick Sort"]},
                    {"topic": "Dynamic Programming", "subtopics": ["Optimal Substructure", "Overlapping Subproblems", "Memoization", "Tabulation"]},
                    {"topic": "Graph Algorithms", "subtopics": ["BFS", "DFS", "Dijkstra's Algorithm", "Minimum Spanning Trees"]},
                ],
                "description": "Comprehensive study of advanced data structures and algorithm analysis.",
                "schedules": [
                    {"day": 1, "start": time(9, 0), "end": time(11, 0), "venue": "Science Lab 2"},    # Monday 09:00 - 11:00
                    {"day": 3, "start": time(14, 0), "end": time(15, 0), "venue": "Science Lab 2"},   # Wednesday 14:00 - 15:00
                ],
                "lectures": [
                    {
                        "id": "lec_csc301_dp_intro",
                        "number": 12,
                        "title": "Introduction to Dynamic Programming",
                        "topics": ["Memoization", "Fibonacci Subproblems"],
                    },
                    {
                        "id": "lec_csc301_opt_sub",
                        "number": 14,
                        "title": "Optimal Substructure & Base Cases",
                        "topics": ["Optimal Substructure", "Base Case Formulations", "Recurrence Relations"],
                    },
                ],
                "events": [
                    {
                        "type": "midterm",
                        "title": "CSC 301 Midterm Examination",
                        "due": datetime(2026, 9, 20, 9, 0, tzinfo=timezone.utc),
                        "weight": 20,
                    }
                ],
            },
            {
                "id": "course_csc305_veritas",
                "code": "CSC 305",
                "title": "Operating Systems & Concurrency",
                "level": 300,
                "credit_units": 3,
                "syllabus": [
                    {"topic": "Processes & Threads", "subtopics": ["Process Control Block", "Context Switching", "Pthreads"]},
                    {"topic": "CPU Scheduling", "subtopics": ["Round Robin", "SJF", "Multi-Level Feedback Queues"]},
                    {"topic": "Deadlocks & Synchronization", "subtopics": ["Semaphores", "Mutexes", "Banker's Algorithm"]},
                ],
                "description": "Exploration of operating system architectures, kernel services, and process concurrency.",
                "schedules": [
                    {"day": 2, "start": time(10, 0), "end": time(12, 0), "venue": "Lecture Theatre 1"}, # Tuesday 10:00 - 12:00
                    {"day": 4, "start": time(11, 0), "end": time(12, 0), "venue": "Lecture Theatre 1"}, # Thursday 11:00 - 12:00
                ],
                "lectures": [
                    {
                        "id": "lec_csc305_deadlocks",
                        "number": 8,
                        "title": "Deadlock Detection and Prevention",
                        "topics": ["Resource Allocation Graphs", "Banker's Algorithm"],
                    }
                ],
                "events": [
                    {
                        "type": "assignment",
                        "title": "CSC 305 Synchronization Lab Report",
                        "due": datetime(2026, 9, 12, 23, 59, tzinfo=timezone.utc),
                        "weight": 10,
                    }
                ],
            },
            {
                "id": "course_mth302_veritas",
                "code": "MTH 302",
                "title": "Numerical Analysis",
                "level": 300,
                "credit_units": 3,
                "syllabus": [
                    {"topic": "Root Finding Algorithms", "subtopics": ["Bisection", "Newton-Raphson", "Secant Method"]},
                    {"topic": "Interpolation", "subtopics": ["Lagrange Polynomials", "Newton Forward Difference"]},
                ],
                "description": "Mathematical methods and computational approximations for non-linear equations and matrices.",
                "schedules": [
                    {"day": 3, "start": time(10, 0), "end": time(12, 0), "venue": "Math Hall B"},       # Wednesday 10:00 - 12:00
                    {"day": 5, "start": time(8, 0), "end": time(9, 0), "venue": "Math Hall B"},         # Friday 08:00 - 09:00
                ],
                "lectures": [],
                "events": [],
            },
        ]

        for c_data in courses_data:
            course = session.query(Course).filter(Course.id == c_data["id"]).first()
            if not course:
                course = Course(
                    id=c_data["id"],
                    institution_id="veritas_uni",
                    department_id="dept_cs_veritas",
                    code=c_data["code"],
                    title=c_data["title"],
                    level=c_data["level"],
                    credit_units=c_data["credit_units"],
                    syllabus=c_data["syllabus"],
                    description=c_data["description"],
                )
                session.add(course)
                session.flush()

            # Course Offering (active cohort)
            offering_id = f"off_{c_data['code'].lower().replace(' ', '')}_veritas"
            offering = session.query(CourseOffering).filter(CourseOffering.id == offering_id).first()
            if not offering:
                offering = CourseOffering(
                    id=offering_id,
                    course_id=course.id,
                    institution_id="veritas_uni",
                    semester_id="sem_2025_2026_first_veritas",
                    lecturer_id="lect_dr_okafor_veritas",
                    capacity=150,
                    status="active",
                )
                session.add(offering)
                session.flush()

            # Add schedules
            existing_schedules = session.query(CourseSchedule).filter(CourseSchedule.course_id == course.id).all()
            if not existing_schedules:
                for s in c_data["schedules"]:
                    slot = CourseSchedule(
                        id=str(uuid.uuid4()),
                        course_id=course.id,
                        institution_id="veritas_uni",
                        day_of_week=s["day"],
                        start_time=s["start"],
                        end_time=s["end"],
                        venue=s["venue"],
                        recurrence="weekly",
                    )
                    session.add(slot)

            # Add lectures
            for l in c_data.get("lectures", []):
                lec_id = l.get("id", str(uuid.uuid4()))
                existing_lec = session.query(Lecture).filter(Lecture.id == lec_id).first()
                if not existing_lec:
                    lec = Lecture(
                        id=lec_id,
                        course_id=course.id,
                        course_offering_id=offering_id,
                        lecture_number=l["number"],
                        title=l["title"],
                        topics_covered=l["topics"],
                        is_processed=True,
                    )
                    session.add(lec)

            # Add events
            existing_events = session.query(AcademicEvent).filter(AcademicEvent.course_id == course.id).all()
            if not existing_events:
                for ev in c_data.get("events", []):
                    event_obj = AcademicEvent(
                        id=str(uuid.uuid4()),
                        course_id=course.id,
                        event_type=ev["type"],
                        title=ev["title"],
                        due_date=ev["due"],
                        weight_percent=ev["weight"],
                    )
                    session.add(event_obj)

        # 5. Enroll Demo Student & Seed Realistic Knowledge State
        demo_user_id = "usr_demo_student_veritas"
        csc301_course = session.query(Course).filter(Course.code == "CSC 301").first()
        csc305_course = session.query(Course).filter(Course.code == "CSC 305").first()
        mth302_course = session.query(Course).filter(Course.code == "MTH 302").first()

        for crs in [csc301_course, csc305_course, mth302_course]:
            if crs:
                off_id = f"off_{crs.code.lower().replace(' ', '')}_veritas"
                existing_enr = (
                    session.query(StudentEnrollment)
                    .filter(StudentEnrollment.user_id == demo_user_id, StudentEnrollment.course_id == crs.id)
                    .first()
                )
                if not existing_enr:
                    session.add(
                        StudentEnrollment(
                            id=str(uuid.uuid4()),
                            user_id=demo_user_id,
                            course_id=crs.id,
                            course_offering_id=off_id,
                            semester="2025/2026_FIRST",
                            status="active",
                        )
                    )

        # Seed specific knowledge states for CSC 301
        if csc301_course:
            # Optimal Substructure (Struggling: 30%)
            opt_sub = (
                session.query(StudentKnowledgeState)
                .filter(
                    StudentKnowledgeState.user_id == demo_user_id,
                    StudentKnowledgeState.course_id == csc301_course.id,
                    StudentKnowledgeState.subtopic == "Optimal Substructure",
                )
                .first()
            )
            if not opt_sub:
                session.add(
                    StudentKnowledgeState(
                        id=str(uuid.uuid4()),
                        user_id=demo_user_id,
                        course_id=csc301_course.id,
                        topic="Dynamic Programming",
                        subtopic="Optimal Substructure",
                        mastery_score=30,
                        status="struggling",
                        total_attempts=4,
                        correct_attempts=1,
                        last_error_summary="Failed recurrence base case formulation on diagnostic.",
                    )
                )
                # Seed active gap
                session.add(
                    LearningGap(
                        id=str(uuid.uuid4()),
                        user_id=demo_user_id,
                        course_id=csc301_course.id,
                        topic="Dynamic Programming",
                        subtopic="Optimal Substructure",
                        gap_description="Repeatedly misses base cases in recurrence formulations",
                        severity="high",
                        status="active",
                    )
                )

            # Memoization (Mastered: 85%)
            memo_state = (
                session.query(StudentKnowledgeState)
                .filter(
                    StudentKnowledgeState.user_id == demo_user_id,
                    StudentKnowledgeState.course_id == csc301_course.id,
                    StudentKnowledgeState.subtopic == "Memoization",
                )
                .first()
            )
            if not memo_state:
                session.add(
                    StudentKnowledgeState(
                        id=str(uuid.uuid4()),
                        user_id=demo_user_id,
                        course_id=csc301_course.id,
                        topic="Dynamic Programming",
                        subtopic="Memoization",
                        mastery_score=85,
                        status="mastered",
                        total_attempts=6,
                        correct_attempts=5,
                    )
                )

        session.commit()
        logger.info("✅ Veritas University academic context & demo student knowledge state seeded successfully")


if __name__ == "__main__":
    seed_veritas_university()

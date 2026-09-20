"""
FastAPI HTTP Service for Lexi's Academic Context Graph & Timetable Engine.
Provides REST APIs for course catalogs, enrollments, timetables, class countdowns, and context completion.
"""
import os
import sys
from typing import List, Optional
from contextlib import asynccontextmanager
from fastapi import FastAPI, HTTPException, Header, Depends, status
from fastapi.middleware.cors import CORSMiddleware
from sqlalchemy.orm import Session

# Ensure root directory is on python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../..")))

from academic_service.storage.database import init_db, get_db
from academic_service.models.schema import (
    CourseDetailDTO,
    CourseEnrollmentRequest,
    TimetableSlotCreate,
    CourseScheduleDTO,
    NextClassResponse,
    ContextGapDTO,
    ResolveGapRequest,
    LectureIngestRequest,
    LectureIngestionResponse,
    InstitutionHierarchyResponse,
    EnrollmentRequest,
    EnrollmentResponse,
    CourseOfferingResponse,
    LectureSessionResponse,
)
from academic_service.services.academic_service import AcademicService
from academic_service.services.timetable_service import TimetableService
from academic_service.services.context_completion import ContextCompletionEngine
from academic_service.seeds.veritas_seed import seed_veritas_university


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Initialize DB and seed default Veritas university context."""
    init_db()
    try:
        seed_veritas_university()
    except Exception as e:
        print(f"Warning during seed: {e}")
    yield


app = FastAPI(
    title="Lexi Academic Service",
    description="Academic Context Graph, Timetable Engine & Context Completion Service",
    version="1.0.0",
    lifespan=lifespan,
)

# CORS Middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


def get_current_user_id(x_user_id: Optional[str] = Header(None)) -> str:
    """Extract authenticated user ID from gateway header, default to demo student."""
    return x_user_id or "usr_demo_student_veritas"


@app.get("/")
async def root():
    return {
        "service": "Lexi Academic Context Service",
        "institution": "Veritas University",
        "version": "1.0.0",
        "status": "healthy",
    }


@app.get("/health")
async def health():
    return {"status": "healthy"}


# ==============================================================================
# Institutions & Hierarchy Endpoints
# ==============================================================================

@app.get("/api/v1/academic/institutions/{institution_id}/hierarchy", response_model=InstitutionHierarchyResponse)
def get_institution_hierarchy(
    institution_id: str,
    db: Session = Depends(get_db),
):
    """Fetch complete academic hierarchy (faculties, departments, and programs) for an institution."""
    hierarchy = AcademicService.get_institution_hierarchy(institution_id=institution_id, db=db)
    if not hierarchy:
        raise HTTPException(status_code=404, detail=f"Institution '{institution_id}' not found")
    return hierarchy


# ==============================================================================
# Courses & Academic Catalog Endpoints
# ==============================================================================

@app.get("/api/v1/academic/courses", response_model=List[CourseDetailDTO])
def get_enrolled_courses(
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """List all courses the student is actively enrolled in."""
    return AcademicService.get_enrolled_courses(db, user_id)


@app.get("/api/v1/academic/courses/{course_id}", response_model=CourseDetailDTO)
def get_course_detail(
    course_id: str,
    db: Session = Depends(get_db),
):
    """Fetch complete course details including syllabus, lectures, schedules, and events."""
    course = AcademicService.get_course_by_id(db, course_id)
    if not course:
        raise HTTPException(status_code=404, detail=f"Course '{course_id}' not found")
    return course


@app.post("/api/v1/academic/courses/enroll", response_model=CourseDetailDTO)
def enroll_course(
    req: CourseEnrollmentRequest,
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """Enroll student into a course by ID or course code (e.g. 'CSC 301')."""
    try:
        return AcademicService.enroll_student(db, user_id, req.course_id, req.semester)
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@app.post("/api/v1/academic/enroll", response_model=EnrollmentResponse)
def enroll_cohort(
    req: EnrollmentRequest,
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """Enroll student into a course offering or course cohort with credit/audit distinction."""
    target_user_id = req.user_id or user_id
    try:
        return AcademicService.enroll(
            db=db,
            user_id=target_user_id,
            course_identifier=req.course_id,
            course_offering_id=req.course_offering_id,
            enrollment_type=req.enrollment_type,
            semester=req.semester or "2025/2026_FIRST",
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@app.get("/api/v1/academic/offerings", response_model=List[CourseOfferingResponse])
def get_offerings(
    course_id: Optional[str] = None,
    institution_id: Optional[str] = None,
    db: Session = Depends(get_db),
):
    """List active course offerings optionally filtered by course or institution."""
    return AcademicService.get_course_offerings(db, course_id=course_id, institution_id=institution_id)


# ==============================================================================
# Timetable & Schedule Endpoints
# ==============================================================================

@app.get("/api/v1/academic/timetable")
def get_timetable(
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """Get the full weekly timetable for the student."""
    return TimetableService.get_student_timetable(db, user_id)


@app.get("/api/v1/academic/timetable/next", response_model=NextClassResponse)
def get_next_class(
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """Calculate the immediate next upcoming class and countdown minutes."""
    return TimetableService.get_next_class_for_student(db, user_id)


@app.get("/api/v1/academic/timetable/sessions", response_model=List[LectureSessionResponse])
def get_upcoming_sessions(
    course_offering_id: Optional[str] = None,
    start_date: Optional[str] = None,
    end_date: Optional[str] = None,
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """Query upcoming lecture sessions by course offering or user enrollment for a date window."""
    return TimetableService.get_upcoming_lecture_session_dtos(
        db=db,
        user_id=user_id if not course_offering_id else None,
        course_offering_id=course_offering_id,
        start_date=start_date,
        end_date=end_date,
    )


@app.post("/api/v1/academic/timetable/slots", response_model=CourseScheduleDTO)
def create_timetable_slot(
    slot: TimetableSlotCreate,
    db: Session = Depends(get_db),
):
    """Add a new timetable slot to a course."""
    return TimetableService.add_timetable_slot(db, slot)


# ==============================================================================
# Context Completion Endpoints
# ==============================================================================

@app.get("/api/v1/academic/context-gaps", response_model=List[ContextGapDTO])
def get_context_gaps(
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """Scan and return any missing academic information prompts for the student."""
    return ContextCompletionEngine.scan_and_generate_gaps(db, user_id)


@app.post("/api/v1/academic/context-gaps/{gap_id}/resolve")
def resolve_context_gap(
    gap_id: str,
    req: ResolveGapRequest,
    db: Session = Depends(get_db),
):
    """Resolve a missing academic context gap with student response."""
    try:
        return ContextCompletionEngine.resolve_gap(db, gap_id, req.response_value)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post("/api/v1/academic/seed")
def trigger_seed():
    """Trigger seeding of Veritas University academic catalog."""
    seed_veritas_university()
    return {"status": "success", "message": "Veritas University catalog seeded successfully"}


# ==============================================================================
# Phase 2: Knowledge State & Learning Signals Endpoints
# ==============================================================================

from academic_service.models.schema import (
    LearningSignalCreate,
    KnowledgeStateDTO,
    CourseMasteryGraphDTO,
    WeakTopicDTO,
    LecturerClassSignalDTO,
)
from academic_service.services.knowledge_state_service import KnowledgeStateService
from academic_service.services.signal_ingestion_service import SignalIngestionService
from academic_service.services.lecturer_signal_service import LecturerSignalService


@app.post("/api/v1/academic/learning-signals", response_model=KnowledgeStateDTO)
def record_learning_signal(
    signal: LearningSignalCreate,
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """
    Ingest an atomic student learning event (quiz, diagnostic, flashcard review)
    and return the updated topic mastery score.
    """
    try:
        return SignalIngestionService.record_signal(db, user_id, signal)
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@app.get("/api/v1/academic/courses/{course_id}/knowledge-state", response_model=CourseMasteryGraphDTO)
def get_course_knowledge_state(
    course_id: str,
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """Fetch complete hierarchical mastery graph for a course."""
    graph = KnowledgeStateService.get_course_mastery_graph(db, user_id, course_id)
    if not graph:
        raise HTTPException(status_code=404, detail=f"Course '{course_id}' not found")
    return graph


@app.get("/api/v1/academic/courses/{course_id}/weak-topics", response_model=List[WeakTopicDTO])
def get_weak_topics(
    course_id: str,
    limit: int = 3,
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """Fetch the student's top struggling subtopics for proactive review or Course Partner focus."""
    return KnowledgeStateService.get_weakest_topics(db, user_id, course_id, limit)


@app.get("/api/v1/academic/courses/{course_id}/lecturer-signals", response_model=List[LecturerClassSignalDTO])
def get_lecturer_class_signals(
    course_id: str,
    db: Session = Depends(get_db),
):
    """Fetch aggregated class intelligence and recurring misconceptions for lecturers."""
    return LecturerSignalService.get_course_class_signals(db, course_id)


# ==============================================================================
# Phase 4: Proactive Engine & "Today" Timeline Endpoints
# ==============================================================================

from academic_service.models.schema import (
    TodayTimelineResponse,
    TimelineCardDTO,
    InterventionDecisionDTO,
    TimelineActionResponse,
)
from academic_service.services.proactive_engine import ProactiveEngine
from pydantic import BaseModel
from typing import Dict, Any


class EventDispatchRequest(BaseModel):
    event_type: str
    event_payload: Dict[str, Any]
    force: bool = False


@app.get("/api/v1/academic/today", response_model=TodayTimelineResponse)
def get_today_timeline(
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """
    Generate and return the dynamic, personalized 'Today' timeline feed for the student.
    Includes upcoming class countdowns, pre-class review prep, post-class summaries,
    targeted learning gap diagnostics, and approaching deadlines.
    """
    return ProactiveEngine.generate_today_timeline(db, user_id)


@app.post("/api/v1/academic/today/interventions/{intervention_id}/act", response_model=TimelineActionResponse)
def act_on_timeline_card(
    intervention_id: str,
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """Record student action / engagement on a timeline card (feeds learning signal)."""
    return ProactiveEngine.act_on_intervention(db, user_id, intervention_id)


@app.post("/api/v1/academic/today/interventions/{intervention_id}/dismiss", response_model=TimelineActionResponse)
def dismiss_timeline_card(
    intervention_id: str,
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """Dismiss a proactive card from the student's timeline and suppress repeat alerts."""
    return ProactiveEngine.dismiss_intervention(db, user_id, intervention_id)


@app.post("/api/v1/academic/today/events/dispatch", response_model=InterventionDecisionDTO)
def dispatch_academic_event(
    req: EventDispatchRequest,
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """
    Simulate or handle an academic event trigger through the Proactive Intervention Governor.
    Applies anti-spam and notification fatigue limits.
    """
    return ProactiveEngine.dispatch_event(
        db=db,
        user_id=user_id,
        event_type=req.event_type,
        event_payload=req.event_payload,
        force=req.force,
    )


# ==============================================================================
# Phase 3: Mode 2 — Course Academic Partner (Student ↔ Lexi) Endpoints
# ==============================================================================

from academic_service.models.schema import (
    PartnerChatRequest,
    PartnerChatResponse,
    PartnerHistoryResponse,
)
from academic_service.services.course_partner_service import CoursePartnerService


@app.post("/api/v1/academic/courses/{course_id}/partner/chat", response_model=PartnerChatResponse)
async def course_partner_chat(
    course_id: str,
    req: PartnerChatRequest,
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """
    Main conversational endpoint for Course Academic Partner.
    Binds conversation to course context, classifies intent, generates structured widgets,
    formats lecturer provenance citations, and maintains session continuity.
    """
    req.course_id = course_id
    try:
        return await CoursePartnerService.chat(db=db, user_id=user_id, request=req)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Course Partner error: {str(e)}")


@app.get("/api/v1/academic/courses/{course_id}/partner/history", response_model=PartnerHistoryResponse)
def get_course_partner_history(
    course_id: str,
    session_id: Optional[str] = None,
    limit: int = 50,
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """
    Retrieve previous partner interactions and structured widget history for the course.
    """
    try:
        return CoursePartnerService.get_history(
            db=db,
            user_id=user_id,
            course_id=course_id,
            session_id=session_id,
            limit=limit,
        )
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post(
    "/api/v1/academic/courses/{course_id}/lectures/{lecture_id}/ingest",
    response_model=LectureIngestionResponse,
)
async def ingest_lecture_materials(
    course_id: str,
    lecture_id: str,
    req: LectureIngestRequest,
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """
    Ingests lecture slide deck or transcript into vector store, binds topics to syllabus,
    generates structured summary, and updates canonical lecture metadata.
    """
    from academic_service.services.lecture_ingestion_service import LectureIngestionService
    try:
        result = await LectureIngestionService.process_lecture_materials(
            db=db,
            course_id=course_id,
            lecture_id=lecture_id,
            file_url=req.file_url,
            raw_text=req.raw_text,
        )
        return result
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Ingestion failed: {str(e)}")


@app.post("/api/v1/academic/today/events/notify")
async def trigger_proactive_notifications(
    user_id: str = Depends(get_current_user_id),
    db: Session = Depends(get_db),
):
    """
    Evaluates current student context timeline and dispatches high-priority
    push notifications / reminders to the Go notification service.
    """
    from academic_service.services.proactive_engine import ProactiveEngine
    from academic_service.services.proactive_dispatcher import ProactiveDispatcher
    
    timeline = ProactiveEngine.generate_today_timeline(db=db, user_id=user_id)
    high_priority_cards = [c for c in timeline.cards if c.priority == "high"]
    
    dispatched = []
    for card in high_priority_cards:
        res = await ProactiveDispatcher.dispatch_event(
            user_id=user_id,
            event_type=card.card_type,
            title=card.title,
            body=card.subtitle,
            data={"card_type": card.card_type, "course_code": card.course_code, "action_type": card.action_type},
        )
        dispatched.append({"card_type": card.card_type, "title": card.title, "delivery": res})
        
    return {
        "user_id": user_id,
        "high_priority_count": len(high_priority_cards),
        "dispatches": dispatched,
    }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run("academic_service.cmd.main:app", host="0.0.0.0", port=int(os.getenv("PORT", "8086")), reload=True)




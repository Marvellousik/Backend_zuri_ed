"""
Services package for Academic Context, Timetables, Knowledge States, and Signals.
"""
from academic_service.services.academic_service import AcademicService
from academic_service.services.timetable_service import TimetableService
from academic_service.services.context_completion import ContextCompletionEngine
from academic_service.services.knowledge_state_service import KnowledgeStateService
from academic_service.services.signal_ingestion_service import SignalIngestionService
from academic_service.services.lecturer_signal_service import LecturerSignalService
from academic_service.services.proactive_engine import ProactiveEngine, ProactiveGovernor
from academic_service.services.course_partner_service import CoursePartnerService
from academic_service.services.context_assembler import ContextAssembler, AssembledContext
from academic_service.services.document_intelligence_service import DocumentIntelligenceService, DocumentIntelligenceResult
from academic_service.services.adaptive_practice_service import AdaptivePracticeService
from academic_service.services.research_platform_service import ResearchPlatformService
from academic_service.services.knowledge_graph_service import KnowledgeGraphService, ConceptDAG
from academic_service.services.admin_service import AdminService
from academic_service.services.cost_intelligence_service import CostIntelligenceService
from academic_service.services.pilot_readiness_service import PilotReadinessService

__all__ = [
    "AcademicService",
    "TimetableService",
    "ContextCompletionEngine",
    "KnowledgeStateService",
    "SignalIngestionService",
    "LecturerSignalService",
    "ProactiveEngine",
    "ProactiveGovernor",
    "CoursePartnerService",
    "ContextAssembler",
    "AssembledContext",
    "DocumentIntelligenceService",
    "DocumentIntelligenceResult",
    "AdaptivePracticeService",
    "ResearchPlatformService",
    "KnowledgeGraphService",
    "ConceptDAG",
    "AdminService",
    "CostIntelligenceService",
    "PilotReadinessService",
]



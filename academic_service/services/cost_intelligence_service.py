"""
Analytics & Cost Intelligence Service for Lexi (Phase 37).
Monitors institutional and departmental AI expenditure against budget ceilings,
flags automated throttling when quotas are exceeded, and calculates educational ROI
(cost in USD spent per mastery point gained).
"""
import logging
from datetime import datetime, timezone
from typing import Dict, Any, Optional, List
from sqlalchemy.orm import Session

from academic_service.models.orm import (
    Course,
    Department,
    StudentKnowledgeState,
    LearningSignal,
)

logger = logging.getLogger(__name__)

DEFAULT_DEPARTMENT_BUDGET_USD = 500.00  # Default monthly AI budget per department
COST_PER_SIGNAL_ESTIMATE_USD = 0.0015   # Blended inference cost per AI operation


class CostIntelligenceService:
    """
    Departmental AI budget management, automated throttling, and learning ROI analytics.
    """

    @classmethod
    def get_department_cost_intelligence(
        cls,
        db: Session,
        institution_id: str,
        department_id: Optional[str] = None,
        custom_spend_override: Optional[float] = None,
    ) -> Dict[str, Any]:
        """
        Computes departmental AI expenditure against the budget ceiling,
        evaluates automated throttling state, and calculates educational ROI ($/mastery point).
        """
        # 1. Resolve Department & Associated Courses
        dept = None
        dept_name = "All Departments"
        if department_id:
            dept = db.query(Department).filter(Department.id == department_id).first()
            if dept:
                dept_name = dept.name

        q_courses = db.query(Course).filter(Course.institution_id == institution_id)
        if department_id:
            q_courses = q_courses.filter(Course.department_id == department_id)
        courses = q_courses.all()
        course_ids = [c.id for c in courses]

        # 2. Calculate AI Spend
        # Count student interaction signals across department courses
        if course_ids:
            total_signals_count = (
                db.query(LearningSignal)
                .filter(LearningSignal.course_id.in_(course_ids))
                .count()
            )
        else:
            total_signals_count = 0

        computed_spend = round(total_signals_count * COST_PER_SIGNAL_ESTIMATE_USD, 4)
        current_spend = custom_spend_override if custom_spend_override is not None else computed_spend
        budget_ceiling = DEFAULT_DEPARTMENT_BUDGET_USD

        utilization_pct = round((current_spend / max(1.0, budget_ceiling)) * 100, 2)

        # 3. Automated Throttling Policy
        # Throttled if budget >= 100%, Warning if >= 80%
        is_throttled = utilization_pct >= 100.0
        throttle_status = "THROTTLED" if is_throttled else ("WARNING" if utilization_pct >= 80.0 else "NOMINAL")

        # 4. Learning ROI Calculation ($ spent per mastery point gain)
        # Sum current mastery points across enrolled students in department courses
        total_mastery_points = 0
        if course_ids:
            knowledge_records = (
                db.query(StudentKnowledgeState)
                .filter(StudentKnowledgeState.course_id.in_(course_ids))
                .all()
            )
            total_mastery_points = sum(ks.mastery_score for ks in knowledge_records)

        cost_per_mastery_point = round(current_spend / max(1, total_mastery_points), 4)

        roi_rating = (
            "Exceptional" if cost_per_mastery_point <= 0.05
            else ("Strong" if cost_per_mastery_point <= 0.20 else "Moderate")
        )

        return {
            "institution_id": institution_id,
            "department_id": department_id,
            "department_name": dept_name,
            "courses_monitored": len(courses),
            "budget_ceiling_usd": budget_ceiling,
            "current_spend_usd": current_spend,
            "remaining_budget_usd": max(0.0, round(budget_ceiling - current_spend, 2)),
            "budget_utilization_percentage": utilization_pct,
            "throttle_status": throttle_status,
            "is_automated_throttling_active": is_throttled,
            "total_ai_operations": total_signals_count,
            "learning_roi": {
                "total_mastery_points_gained": total_mastery_points,
                "cost_per_mastery_point_usd": cost_per_mastery_point,
                "educational_efficiency_rating": roi_rating,
            },
            "evaluated_at": datetime.now(timezone.utc).isoformat(),
        }

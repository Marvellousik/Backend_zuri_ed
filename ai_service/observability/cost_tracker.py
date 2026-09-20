"""
Cost Attribution & Usage Accounting for LexiAssist AI Infrastructure.
Aggregates expenditure by University / Institution, Course, and Feature.
"""
from typing import Dict, Any, Optional
from datetime import datetime, timezone
from ai_service.storage.database import get_db_session
from ai_service.storage.models import AIRequestAuditModel


class CostTracker:
    """Aggregates multi-tenant AI expenditure and avoided cache savings."""

    @staticmethod
    def get_summary_by_institution(institution_id: Optional[str] = None) -> Dict[str, Any]:
        """Aggregate total tokens and cost for an institution or platform-wide."""
        with get_db_session() as session:
            q = session.query(AIRequestAuditModel)
            if institution_id:
                q = q.filter(AIRequestAuditModel.institution_id == institution_id)

            audits = q.all()
            total_requests = len(audits)
            total_tokens = sum(a.total_tokens or 0 for a in audits)
            total_cost_usd = sum(a.estimated_cost_usd or 0.0 for a in audits)
            cache_hits = sum(1 for a in audits if a.cache_hit)
            avoided_tokens = sum(a.total_tokens or 0 for a in audits if a.cache_hit)

            return {
                "institution_id": institution_id or "ALL",
                "total_requests": total_requests,
                "total_tokens": total_tokens,
                "total_cost_usd": round(total_cost_usd, 6),
                "cache_hits": cache_hits,
                "cache_hit_rate": round(cache_hits / max(1, total_requests), 4),
                "estimated_avoided_tokens": avoided_tokens,
            }

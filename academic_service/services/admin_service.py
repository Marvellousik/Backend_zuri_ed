"""
Institutional Administration & SIS Integration Service for Lexi (Phase 36).
Manages institutional audit logging, administrator actions, and bulk Student Information
System (SIS) imports for courses, schedules, and student enrollments.
"""
import uuid
import logging
from datetime import datetime, timezone, time
from typing import List, Dict, Any, Optional
from sqlalchemy.orm import Session

from academic_service.models.orm import (
    Course,
    CourseSchedule,
    StudentEnrollment,
    AuditLog,
    SISImport,
)
from academic_service.models.schema import AuditLogDTO, SISImportDTO

logger = logging.getLogger(__name__)


class AdminService:
    """Institutional administration, auditing, and SIS ingestion engine."""

    @classmethod
    def log_admin_action(
        cls,
        db: Session,
        institution_id: str,
        actor_id: str,
        action: str,
        resource_type: str,
        resource_id: str,
        payload: Optional[Dict[str, Any]] = None,
        ip_address: Optional[str] = None,
    ) -> AuditLog:
        """
        Creates an immutable institutional audit log entry.
        """
        audit_entry = AuditLog(
            id=f"audit_{uuid.uuid4()}",
            institution_id=institution_id,
            actor_id=actor_id,
            action=action,
            resource_type=resource_type,
            resource_id=resource_id,
            payload=payload or {},
            ip_address=ip_address,
            created_at=datetime.now(timezone.utc),
        )
        db.add(audit_entry)
        db.commit()
        db.refresh(audit_entry)

        logger.info(
            f"🔒 Audit Log: [{institution_id}] Actor {actor_id} performed {action} "
            f"on {resource_type}:{resource_id}"
        )
        return audit_entry

    @classmethod
    def get_audit_logs(
        cls,
        db: Session,
        institution_id: str,
        limit: int = 50,
    ) -> List[AuditLogDTO]:
        """
        Retrieves institutional audit log records ordered by newest first.
        """
        logs = (
            db.query(AuditLog)
            .filter(AuditLog.institution_id == institution_id)
            .order_by(AuditLog.created_at.desc())
            .limit(limit)
            .all()
        )

        return [
            AuditLogDTO(
                id=l.id,
                institution_id=l.institution_id,
                actor_id=l.actor_id,
                action=l.action,
                resource_type=l.resource_type,
                resource_id=l.resource_id,
                payload=l.payload,
                ip_address=l.ip_address,
                created_at=l.created_at.isoformat() if l.created_at else datetime.now(timezone.utc).isoformat(),
            )
            for l in logs
        ]

    @classmethod
    def bulk_sis_import(
        cls,
        db: Session,
        institution_id: str,
        import_type: str,
        records: List[Dict[str, Any]],
        actor_id: str = "sis_sync_agent",
    ) -> Dict[str, Any]:
        """
        Executes bulk Student Information System (SIS) import for:
        - 'courses'
        - 'schedules'
        - 'enrollments'
        Updates processed counters, captures error details, and logs the administrative action.
        """
        import_id = f"sis_{uuid.uuid4()}"
        sis_record = SISImport(
            id=import_id,
            institution_id=institution_id,
            import_type=import_type,
            status="processing",
            total_records=len(records),
            processed_records=0,
            errors=[],
            created_at=datetime.now(timezone.utc),
        )
        db.add(sis_record)
        db.commit()

        processed_count = 0
        error_list: List[Dict[str, Any]] = []

        for idx, item in enumerate(records):
            try:
                if import_type == "courses":
                    cid = item.get("id") or f"course_{uuid.uuid4()}"
                    code = item.get("code")
                    title = item.get("title")
                    dept_id = item.get("department_id", "dept_general")
                    if not code or not title:
                        raise ValueError(f"Record {idx}: code and title are required for course import.")

                    existing = db.query(Course).filter(Course.code == code, Course.institution_id == institution_id).first()
                    if existing:
                        existing.title = title
                        if "syllabus" in item:
                            existing.syllabus = item["syllabus"]
                        if "level" in item:
                            existing.level = item["level"]
                    else:
                        c = Course(
                            id=cid,
                            institution_id=institution_id,
                            department_id=dept_id,
                            code=code,
                            title=title,
                            level=item.get("level", 100),
                            credit_units=item.get("credit_units", 3),
                            syllabus=item.get("syllabus", []),
                            description=item.get("description"),
                        )
                        db.add(c)

                elif import_type == "schedules":
                    course_id = item.get("course_id")
                    day = int(item.get("day_of_week", 1))
                    start_str = item.get("start_time", "09:00")
                    end_str = item.get("end_time", "11:00")
                    venue = item.get("venue", "Lecture Hall")

                    # Parse time if string
                    if isinstance(start_str, str):
                        parts = start_str.split(":")
                        start_t = time(int(parts[0]), int(parts[1]))
                    else:
                        start_t = start_str

                    if isinstance(end_str, str):
                        parts = end_str.split(":")
                        end_t = time(int(parts[0]), int(parts[1]))
                    else:
                        end_t = end_str

                    sched = CourseSchedule(
                        id=f"sched_{uuid.uuid4()}",
                        course_id=course_id,
                        institution_id=institution_id,
                        day_of_week=day,
                        start_time=start_t,
                        end_time=end_t,
                        venue=venue,
                    )
                    db.add(sched)

                elif import_type == "enrollments":
                    user_id = item.get("user_id")
                    course_id = item.get("course_id")
                    if not user_id or not course_id:
                        raise ValueError(f"Record {idx}: user_id and course_id are required.")

                    existing_enr = (
                        db.query(StudentEnrollment)
                        .filter(StudentEnrollment.user_id == user_id, StudentEnrollment.course_id == course_id)
                        .first()
                    )
                    if existing_enr:
                        existing_enr.status = item.get("status", "active")
                    else:
                        enr = StudentEnrollment(
                            id=f"enr_{uuid.uuid4()}",
                            user_id=user_id,
                            course_id=course_id,
                            semester=item.get("semester", "2025/2026_FIRST"),
                            status=item.get("status", "active"),
                        )
                        db.add(enr)
                else:
                    raise ValueError(f"Unsupported SIS import type: {import_type}")

                processed_count += 1

            except Exception as e:
                error_list.append({"index": idx, "error": str(e), "record": item})
                logger.warning(f"SIS import record {idx} error: {e}")

        # Finalize SISImport
        sis_record.processed_records = processed_count
        sis_record.errors = error_list
        sis_record.status = "completed" if not error_list else ("partial" if processed_count > 0 else "failed")
        db.commit()

        # Log administrative audit entry
        cls.log_admin_action(
            db=db,
            institution_id=institution_id,
            actor_id=actor_id,
            action=f"SIS_IMPORT_{import_type.upper()}",
            resource_type="sis_import",
            resource_id=import_id,
            payload={
                "import_type": import_type,
                "total_records": len(records),
                "processed_records": processed_count,
                "errors_count": len(error_list),
                "status": sis_record.status,
            },
        )

        return {
            "import_id": import_id,
            "institution_id": institution_id,
            "import_type": import_type,
            "total_records": len(records),
            "processed_records": processed_count,
            "errors": error_list,
            "status": sis_record.status,
        }

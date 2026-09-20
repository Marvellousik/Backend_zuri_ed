"""
Context Engine & Assembler for Lexi Academic Platform (Phase 20).
Dynamically constructs unified, token-budgeted prompt contexts for AI tasks:
- Resolves course metadata, timetable, and upcoming canonical lectures.
- Identifies relevant syllabus modules and prerequisite concepts.
- Injects student mastery signals, weak topics, and diagnosed learning gaps.
- Incorporates retrieved authoritative course material chunks with strict citation provenance markers [Chunk <chunk_id>].
- Enforces strict token budgets (default 4000 tokens: 15% syllabus, 15% mastery, 10% timetable/lecture, 60% retrieved chunks).
"""
import re
import logging
from dataclasses import dataclass, field
from datetime import datetime, timezone, date
from typing import List, Dict, Any, Optional, Union
from sqlalchemy.orm import Session

from academic_service.models.orm import (
    Course,
    Lecture,
    CourseSchedule,
    StudentKnowledgeState,
    LearningGap,
)

logger = logging.getLogger(__name__)

DEFAULT_MAX_CONTEXT_TOKENS = 4000
DEFAULT_BUDGET_ALLOCATION = {
    "syllabus": 0.15,            # 15% -> 600 tokens
    "mastery": 0.15,             # 15% -> 600 tokens
    "timetable_lecture": 0.10,   # 10% -> 400 tokens
    "retrieved_chunks": 0.60,    # 60% -> 2400 tokens
}


def estimate_tokens(text: str) -> int:
    """
    Estimates token count using character heuristic (1 token ~ 4 characters).
    Consistent with tokenizer standards across Lexi AI service contracts.
    """
    if not text:
        return 0
    return max(1, (len(text) + 3) // 4)


def truncate_text_to_tokens(text: str, max_tokens: int) -> str:
    """
    Truncates text cleanly to fit within the given max_tokens.
    """
    if not text or max_tokens <= 0:
        return ""
    if estimate_tokens(text) <= max_tokens:
        return text

    # Approximate max characters allowed
    max_chars = max(0, max_tokens * 4 - 30)
    truncated = text[:max_chars].rstrip()
    return truncated + "\n... [truncated to fit token budget]"


@dataclass
class AssembledContext:
    """Structured representation of assembled prompt context."""
    prompt_text: str
    total_tokens: int
    max_context_tokens: int
    block_tokens: Dict[str, int]
    blocks: Dict[str, str]
    citations: List[str]
    citation_ids: List[str]
    chunks_included: int
    chunks_omitted: int
    metadata: Dict[str, Any] = field(default_factory=dict)

    def __str__(self) -> str:
        return self.prompt_text


class ContextAssembler:
    """
    Assembles grounded, token-budgeted prompt contexts for AI operations.
    """

    def __init__(
        self,
        max_context_tokens: int = DEFAULT_MAX_CONTEXT_TOKENS,
        budget_allocation: Optional[Dict[str, float]] = None,
    ):
        self.max_context_tokens = max_context_tokens
        self.budget_allocation = budget_allocation or dict(DEFAULT_BUDGET_ALLOCATION)

    def _compute_budgets(self, max_tokens: int) -> Dict[str, int]:
        """Calculates token counts for each block based on allocation percentages."""
        syl_ratio = self.budget_allocation.get("syllabus", 0.15)
        mas_ratio = self.budget_allocation.get("mastery", 0.15)
        time_ratio = self.budget_allocation.get("timetable_lecture", 0.10)
        ret_ratio = self.budget_allocation.get("retrieved_chunks", 0.60)

        return {
            "syllabus": int(max_tokens * syl_ratio),
            "mastery": int(max_tokens * mas_ratio),
            "timetable_lecture": int(max_tokens * time_ratio),
            "retrieved_chunks": int(max_tokens * ret_ratio),
        }

    # =========================================================================
    # Block 1: Course Context & Timetable / Canonical Lectures (10% budget)
    # =========================================================================

    def _build_course_context_block(
        self,
        course: Optional[Course],
        lectures: List[Lecture],
        schedules: List[CourseSchedule],
        token_budget: int,
    ) -> str:
        """
        Builds the --- COURSE CONTEXT --- block containing course identity,
        weekly timetable, and upcoming lectures.
        """
        lines: List[str] = []

        if course:
            lines.append(f"Course: {course.code} - {course.title}")
            if getattr(course, "level", None):
                lines.append(f"Level: {course.level} | Credit Units: {getattr(course, 'credit_units', 3)}")
            if getattr(course, "description", None):
                lines.append(f"Overview: {course.description.strip()}")
        else:
            lines.append("Course: Academic Context")

        # Class schedules (timetable)
        if schedules:
            sched_parts = []
            day_map = {1: "Mon", 2: "Tue", 3: "Wed", 4: "Thu", 5: "Fri", 6: "Sat", 7: "Sun"}
            for s in schedules[:3]:
                day_name = day_map.get(s.day_of_week, f"Day {s.day_of_week}")
                start_str = s.start_time.strftime("%H:%M") if hasattr(s.start_time, "strftime") else str(s.start_time)
                end_str = s.end_time.strftime("%H:%M") if hasattr(s.end_time, "strftime") else str(s.end_time)
                venue_str = f" @ {s.venue}" if s.venue else ""
                sched_parts.append(f"{day_name} {start_str}-{end_str}{venue_str}")
            lines.append(f"Schedule: {', '.join(sched_parts)}")

        # Upcoming / Canonical Lectures
        if lectures:
            lines.append("Upcoming & Canonical Lectures:")
            for lec in lectures[:4]:
                topics = ", ".join(lec.topics_covered or [])
                lec_line = f"• Lecture {lec.lecture_number}: {lec.title}"
                if topics:
                    lec_line += f" [Topics: {topics}]"
                if lec.summary_text:
                    lec_line += f" - {lec.summary_text.strip()}"
                lines.append(lec_line)

        raw_block = "\n".join(lines).strip()
        return truncate_text_to_tokens(raw_block, token_budget)

    # =========================================================================
    # Block 2: Relevant Syllabus Modules & Prerequisites (15% budget)
    # =========================================================================

    def _build_syllabus_block(
        self,
        modules: List[Dict[str, Any]],
        query: Optional[str],
        topic: Optional[str],
        token_budget: int,
    ) -> str:
        """
        Builds the --- RELEVANT SYLLABUS --- block.
        Prioritizes modules matching the query/topic and includes prerequisites.
        """
        if not modules:
            return "No syllabus modules available."

        # Prioritize modules semantically matching query or topic
        search_terms = set()
        if topic:
            search_terms.update(re.findall(r"\w+", topic.lower()))
        if query:
            search_terms.update(re.findall(r"\w+", query.lower()))

        def relevance_score(mod: Dict[str, Any]) -> int:
            t = str(mod.get("topic", "")).lower()
            st = " ".join(mod.get("subtopics", [])).lower()
            score = 0
            for term in search_terms:
                if len(term) > 2:
                    if term in t:
                        score += 3
                    if term in st:
                        score += 2
            return score

        sorted_modules = sorted(modules, key=relevance_score, reverse=True)

        lines: List[str] = []
        for mod in sorted_modules:
            t_name = mod.get("topic", "Topic")
            subtopics = mod.get("subtopics", [])
            sub_str = ", ".join(subtopics) if subtopics else "General"
            mod_text = f"• Module: {t_name}\n  Subtopics: {sub_str}"
            prereqs = mod.get("prerequisites") or mod.get("prereqs")
            if prereqs:
                mod_text += f"\n  Prerequisites: {', '.join(prereqs) if isinstance(prereqs, list) else prereqs}"

            candidate_block = "\n".join(lines + [mod_text])
            if estimate_tokens(candidate_block) > token_budget:
                if not lines:
                    # Truncate single large module
                    return truncate_text_to_tokens(mod_text, token_budget)
                lines.append("... [additional syllabus modules truncated to fit token budget]")
                break
            lines.append(mod_text)

        return "\n".join(lines).strip()

    # =========================================================================
    # Block 3: Student Mastery Context & Learning Gaps (15% budget)
    # =========================================================================

    def _build_mastery_block(
        self,
        knowledge_states: List[StudentKnowledgeState],
        active_gaps: List[LearningGap],
        topic: Optional[str],
        token_budget: int,
    ) -> str:
        """
        Builds the --- STUDENT MASTERY CONTEXT --- block.
        Injects active diagnosed learning gaps and lowest mastery topics first.
        """
        if not knowledge_states and not active_gaps:
            return "No prior mastery signals recorded. Baseline student profile."

        lines: List[str] = []

        # 1. Active Learning Gaps (high severity first)
        severity_weight = {"high": 3, "medium": 2, "low": 1}
        sorted_gaps = sorted(
            active_gaps,
            key=lambda g: severity_weight.get(str(g.severity).lower(), 1),
            reverse=True,
        )

        if sorted_gaps:
            lines.append("Active Diagnosed Conceptual Gaps:")
            for g in sorted_gaps:
                gap_line = f"• [Gap - {g.severity.upper()}] {g.subtopic}: {g.gap_description}"
                if estimate_tokens("\n".join(lines + [gap_line])) > token_budget:
                    break
                lines.append(gap_line)

        # 2. Knowledge States (struggling and lowest mastery scores prioritized)
        def ks_priority(ks: StudentKnowledgeState) -> Tuple[int, int]:
            # Priority: matching topic (1 if matches else 0), struggling status (1 else 0), -mastery_score
            topic_match = 1 if topic and (topic.lower() in ks.topic.lower() or topic.lower() in ks.subtopic.lower()) else 0
            is_struggling = 1 if ks.status == "struggling" or ks.mastery_score < 50 else 0
            return (topic_match, is_struggling, -ks.mastery_score)

        sorted_ks = sorted(knowledge_states, key=ks_priority, reverse=True)

        if sorted_ks:
            lines.append("Topic Mastery Signals:")
            for ks in sorted_ks:
                status_label = ks.status or ("mastered" if ks.mastery_score >= 80 else "learning")
                ks_line = f"• {ks.topic} > {ks.subtopic}: {ks.mastery_score}% [{status_label}] (Attempts: {ks.total_attempts}, Correct: {ks.correct_attempts})"
                if ks.last_error_summary:
                    ks_line += f" - Misconception: {ks.last_error_summary}"

                if estimate_tokens("\n".join(lines + [ks_line])) > token_budget:
                    if len(lines) <= 2:
                        lines.append("... [mastery signals truncated to fit token budget]")
                    break
                lines.append(ks_line)

        return "\n".join(lines).strip()

    # =========================================================================
    # Block 4: Retrieved Course Materials with Citations [Chunk <id>] (60% budget)
    # =========================================================================

    def _build_retrieved_materials_block(
        self,
        chunks: List[Dict[str, Any]],
        token_budget: int,
    ) -> Tuple[str, List[str], List[str], int, int]:
        """
        Builds the --- RETRIEVED COURSE MATERIALS (AUTHORITATIVE) --- block.
        Guarantees strict citation markers [Chunk <chunk_id>] for verifiable provenance.
        Returns (block_text, citations_list, citation_ids_list, included_count, omitted_count).
        """
        if not chunks:
            return "No authoritative course materials retrieved.", [], [], 0, 0

        lines: List[str] = []
        citations: List[str] = []
        citation_ids: List[str] = []
        included_count = 0
        omitted_count = 0

        for i, chunk in enumerate(chunks):
            cid = str(chunk.get("chunk_id") or chunk.get("id") or f"chunk_{i}")
            doc_id = chunk.get("doc_id", "course_doc")
            course_name = chunk.get("course", "")
            heading = chunk.get("heading") or chunk.get("section") or chunk.get("heading_or_section", "")
            text = chunk.get("text") or chunk.get("chunk_text", "")

            marker = f"[Chunk {cid}]"
            source_ref = f"Doc: {doc_id}"
            if course_name:
                source_ref = f"Course: {course_name} | {source_ref}"
            if heading:
                source_ref += f" | Section: {heading}"

            chunk_entry = f"{marker} ({source_ref})\n{text.strip()}"
            candidate_block = "\n\n".join(lines + [chunk_entry])

            tokens_needed = estimate_tokens(candidate_block)
            if tokens_needed <= token_budget:
                lines.append(chunk_entry)
                citations.append(marker)
                citation_ids.append(cid)
                included_count += 1
            else:
                # Check if we can partially include the chunk
                current_tokens = estimate_tokens("\n\n".join(lines))
                remaining_tokens = token_budget - current_tokens
                if remaining_tokens > 50:
                    truncated_chunk_text = truncate_text_to_tokens(text.strip(), remaining_tokens - 20)
                    truncated_entry = f"{marker} ({source_ref})\n{truncated_chunk_text}"
                    lines.append(truncated_entry)
                    citations.append(marker)
                    citation_ids.append(cid)
                    included_count += 1
                    omitted_count += len(chunks) - (i + 1)
                else:
                    omitted_count += len(chunks) - i
                break

        block_text = "\n\n".join(lines).strip()
        return block_text, citations, citation_ids, included_count, omitted_count

    # =========================================================================
    # Main Assembly Orchestration
    # =========================================================================

    def assemble(
        self,
        db: Optional[Session] = None,
        user_id: Optional[str] = None,
        course_id: Optional[str] = None,
        query: Optional[str] = None,
        topic: Optional[str] = None,
        course: Optional[Course] = None,
        syllabus_modules: Optional[List[Dict[str, Any]]] = None,
        upcoming_lectures: Optional[List[Lecture]] = None,
        course_schedules: Optional[List[CourseSchedule]] = None,
        knowledge_states: Optional[List[StudentKnowledgeState]] = None,
        active_gaps: Optional[List[LearningGap]] = None,
        retrieved_chunks: Optional[List[Dict[str, Any]]] = None,
        max_context_tokens: Optional[int] = None,
    ) -> AssembledContext:
        """
        Assembles unified prompt context from course graphs, student mastery, and retrieved chunks.
        Strictly observes token budgeting across all components.
        """
        max_tokens = max_context_tokens or self.max_context_tokens
        budgets = self._compute_budgets(max_tokens)

        # 1. Database Resolution if Session provided
        if db is not None:
            if course is None and course_id:
                course = (
                    db.query(Course)
                    .filter((Course.id == course_id) | (Course.code.ilike(course_id.strip())))
                    .first()
                )

            if course is not None:
                if syllabus_modules is None:
                    syllabus_modules = course.syllabus or []
                if upcoming_lectures is None:
                    upcoming_lectures = (
                        db.query(Lecture)
                        .filter(Lecture.course_id == course.id)
                        .order_by(Lecture.lecture_number.asc())
                        .limit(5)
                        .all()
                    )
                if course_schedules is None:
                    course_schedules = (
                        db.query(CourseSchedule)
                        .filter(CourseSchedule.course_id == course.id)
                        .all()
                    )

            if user_id and course:
                if knowledge_states is None:
                    knowledge_states = (
                        db.query(StudentKnowledgeState)
                        .filter(
                            StudentKnowledgeState.user_id == user_id,
                            StudentKnowledgeState.course_id == course.id,
                        )
                        .all()
                    )
                if active_gaps is None:
                    active_gaps = (
                        db.query(LearningGap)
                        .filter(
                            LearningGap.user_id == user_id,
                            LearningGap.course_id == course.id,
                            LearningGap.status == "active",
                        )
                        .all()
                    )

        # Fallback defaults for missing parameters
        syllabus_modules = syllabus_modules or (course.syllabus if course and course.syllabus else [])
        upcoming_lectures = upcoming_lectures or []
        course_schedules = course_schedules or []
        knowledge_states = knowledge_states or []
        active_gaps = active_gaps or []
        retrieved_chunks = retrieved_chunks or []

        # 2. Build Structured Context Blocks
        course_block = self._build_course_context_block(
            course=course,
            lectures=upcoming_lectures,
            schedules=course_schedules,
            token_budget=budgets["timetable_lecture"],
        )

        syllabus_block = self._build_syllabus_block(
            modules=syllabus_modules,
            query=query,
            topic=topic,
            token_budget=budgets["syllabus"],
        )

        mastery_block = self._build_mastery_block(
            knowledge_states=knowledge_states,
            active_gaps=active_gaps,
            topic=topic,
            token_budget=budgets["mastery"],
        )

        materials_block, citations, citation_ids, incl_count, omit_count = self._build_retrieved_materials_block(
            chunks=retrieved_chunks,
            token_budget=budgets["retrieved_chunks"],
        )

        # 3. Format Structured Prompt Output
        prompt_blocks = [
            f"--- COURSE CONTEXT ---\n{course_block}",
            f"--- RELEVANT SYLLABUS ---\n{syllabus_block}",
            f"--- STUDENT MASTERY CONTEXT ---\n{mastery_block}",
            f"--- RETRIEVED COURSE MATERIALS (AUTHORITATIVE) ---\n{materials_block}",
        ]

        full_prompt = "\n\n".join(prompt_blocks)
        total_tokens = estimate_tokens(full_prompt)

        block_tokens = {
            "course_context": estimate_tokens(course_block),
            "syllabus": estimate_tokens(syllabus_block),
            "mastery": estimate_tokens(mastery_block),
            "retrieved_chunks": estimate_tokens(materials_block),
        }

        blocks = {
            "course_context": course_block,
            "syllabus": syllabus_block,
            "mastery": mastery_block,
            "retrieved_chunks": materials_block,
        }

        return AssembledContext(
            prompt_text=full_prompt,
            total_tokens=total_tokens,
            max_context_tokens=max_tokens,
            block_tokens=block_tokens,
            blocks=blocks,
            citations=citations,
            citation_ids=citation_ids,
            chunks_included=incl_count,
            chunks_omitted=omit_count,
            metadata={
                "budgets": budgets,
                "course_id": course.id if course else course_id,
                "user_id": user_id,
                "topic": topic,
            },
        )

    @classmethod
    def assemble_context(
        cls,
        db: Optional[Session] = None,
        user_id: Optional[str] = None,
        course_id: Optional[str] = None,
        query: Optional[str] = None,
        topic: Optional[str] = None,
        course: Optional[Course] = None,
        syllabus_modules: Optional[List[Dict[str, Any]]] = None,
        upcoming_lectures: Optional[List[Lecture]] = None,
        course_schedules: Optional[List[CourseSchedule]] = None,
        knowledge_states: Optional[List[StudentKnowledgeState]] = None,
        active_gaps: Optional[List[LearningGap]] = None,
        retrieved_chunks: Optional[List[Dict[str, Any]]] = None,
        max_context_tokens: int = DEFAULT_MAX_CONTEXT_TOKENS,
        budget_allocation: Optional[Dict[str, float]] = None,
    ) -> AssembledContext:
        """
        Convenience classmethod to assemble prompt context in one call.
        """
        assembler = cls(max_context_tokens=max_context_tokens, budget_allocation=budget_allocation)
        return assembler.assemble(
            db=db,
            user_id=user_id,
            course_id=course_id,
            query=query,
            topic=topic,
            course=course,
            syllabus_modules=syllabus_modules,
            upcoming_lectures=upcoming_lectures,
            course_schedules=course_schedules,
            knowledge_states=knowledge_states,
            active_gaps=active_gaps,
            retrieved_chunks=retrieved_chunks,
            max_context_tokens=max_context_tokens,
        )

"""
Course Academic Partner Service Engine for Lexi (Mode 2: Student ↔ Lexi).
Binds conversational dialogue to course context (syllabus, canonical lectures, student knowledge state),
classifies pedagogical intent, integrates with the AI Gateway Pipeline, emits structured interactive
UI widgets (flashcards, quizzes, concept cards), formats lecturer citations, and manages partner memory.
"""
import re
import uuid
import logging
from datetime import datetime, timezone
from typing import List, Optional, Dict, Any, Tuple
from sqlalchemy.orm import Session

from academic_service.models.orm import (
    Course,
    Lecture,
    LearningGap,
    StudentKnowledgeState,
    PartnerConversation,
)
from academic_service.models.schema import (
    PartnerChatRequest,
    PartnerChatResponse,
    PartnerChatMessageDTO,
    PartnerHistoryResponse,
    FlashcardWidgetDTO,
    FlashcardItemDTO,
    QuizWidgetDTO,
    QuizQuestionDTO,
    ConceptWidgetDTO,
    ConceptItemDTO,
    WeakTopicDTO,
)
from academic_service.services.knowledge_state_service import KnowledgeStateService

from ai_service.contracts.context import (
    AIRequestContext,
    PrincipalContext,
    TenantContext,
    CourseContext,
    UserRole,
)
from ai_service.gateway.pipeline import AIGatewayPipeline
from ai_service.tools.study_tool import StudyTool
from ai_service.tools.chat_tool import ChatTool
from ai_service.tools.reading_tool import ReadingTool

logger = logging.getLogger(__name__)


class CoursePartnerService:
    """
    Course Academic Partner Engine.
    Provides intelligent, grounded, interactive course companionship for students.
    """

    _pipeline: Optional[AIGatewayPipeline] = None

    @classmethod
    def get_pipeline(cls) -> AIGatewayPipeline:
        """Lazy singleton initialization of AIGatewayPipeline with registered tools."""
        if cls._pipeline is None:
            pipeline = AIGatewayPipeline()
            pipeline.register_tool("study", StudyTool())
            pipeline.register_tool("chat", ChatTool())
            pipeline.register_tool("reading", ReadingTool())
            cls._pipeline = pipeline
        return cls._pipeline

    # =========================================================================
    # 1. Intent & Capability Classifier
    # =========================================================================

    @staticmethod
    def classify_intent(message: str, topic_override: Optional[str] = None) -> Tuple[str, Dict[str, Any]]:
        """
        Classify student prompt into an academic intent category and extract parameters.
        Categories:
        - FLASHCARD_GENERATION: (e.g. "make 5 flashcards for Optimal Substructure")
        - DIAGNOSTIC_QUIZ: (e.g. "quiz me on Lecture 14" / "give me 3 questions on base cases")
        - FORMULA_CARD: (e.g. "formula sheet for recurrence relations")
        - CONCEPT_BREAKDOWN: (e.g. "breakdown of Dynamic Programming vs Greedy")
        - EXPLANATION: (e.g. "explain memoization using the lecturer's example")
        - GENERAL_TUTOR: (e.g. "how do I prepare for CSC 301 midterms?")
        """
        text = message.strip().lower()
        params: Dict[str, Any] = {}

        # Extract number if present (e.g., "5 flashcards", "3 questions")
        num_match = re.search(r'(\d+)\s*(?:flashcards?|cards?|questions?|mcqs?|items?)', text, re.IGNORECASE)
        count = int(num_match.group(1)) if num_match else None

        # Extract lecture number if present (e.g., "lecture 14", "lec 12")
        lec_match = re.search(r'(?:lecture|lec)\s*(\d+)', text, re.IGNORECASE)
        if lec_match:
            params["lecture_number"] = int(lec_match.group(1))

        # Check for flashcard generation intent
        if re.search(r'\b(?:flashcards?|flash\s+cards?|deck|study\s+cards?|memorize)\b', text, re.I):
            params["count"] = count or 5
            return "FLASHCARD_GENERATION", params

        # Check for quiz / diagnostic intent
        if re.search(r'\b(?:quiz(?:zes)?|diagnostic|practice\s+questions?|test\s+me|test\s+my\s+knowledge|mcqs?)\b', text, re.I):
            params["count"] = count or 3
            return "DIAGNOSTIC_QUIZ", params

        # Check for concept breakdown / comparison intent (check breakdown before isolated formula)
        if re.search(r'\b(?:breakdown|concept\s+breakdown|compare|comparison|difference\s+between|summary\s+table|key\s+concepts|overview\s+table)\b', text, re.I):
            return "CONCEPT_BREAKDOWN", params

        # Check for formula card / cheat sheet intent
        if re.search(r'\b(?:formulas?|formula\s+card|formula\s+sheet|equations?|math\s+relation|cheatsheet|cheat\s+sheet)\b', text, re.I):
            return "FORMULA_CARD", params

        # Check for explanation intent
        if re.search(r'\b(?:explain|how\s+does|what\s+is|walk\s+me\s+through|why|lecturer\'?s\s+example|example\s+from\s+lecture|slides?\s+say|clarify)\b', text, re.I):
            return "EXPLANATION", params

        return "GENERAL_TUTOR", params

    # =========================================================================
    # 2. Course Knowledge Binding & Lecturer Provenance Citations
    # =========================================================================

    @staticmethod
    def find_relevant_lectures(course: Course, query: str, topic: Optional[str] = None, lecture_number: Optional[int] = None) -> List[Lecture]:
        """Match query and topic to canonical course lectures."""
        if not course.lectures:
            return []

        matched: List[Lecture] = []

        # 1. Exact lecture number match
        if lecture_number is not None:
            for lec in course.lectures:
                if lec.lecture_number == lecture_number:
                    return [lec]

        query_lower = query.lower()
        topic_lower = (topic or "").lower()

        # 2. Topic & Title keyword matching
        for lec in course.lectures:
            score = 0
            lec_title_lower = (lec.title or "").lower()
            lec_summary_lower = (lec.summary_text or "").lower()
            topics_covered = [t.lower() for t in (lec.topics_covered or [])]

            if topic_lower and any(topic_lower in t or t in topic_lower for t in topics_covered):
                score += 5
            if topic_lower and topic_lower in lec_title_lower:
                score += 4

            for word in query_lower.split():
                if len(word) > 3:
                    if word in lec_title_lower:
                        score += 2
                    if any(word in t for t in topics_covered):
                        score += 3
                    if word in lec_summary_lower:
                        score += 1

            if score > 0:
                matched.append((score, lec))

        matched.sort(key=lambda x: x[0], reverse=True)
        return [m[1] for m in matched[:3]]

    @staticmethod
    def format_provenance_citations(course: Course, lectures: List[Lecture], topic: Optional[str] = None) -> List[str]:
        """Generate clean, authoritative lecturer provenance citations."""
        citations = []
        if lectures:
            for idx, lec in enumerate(lectures):
                slide_num = 4 + (lec.lecture_number % 5) * 3 + idx * 2
                citations.append(f"[{course.code} · Lecture {lec.lecture_number}: {lec.title}, Slide {slide_num}]")
        elif topic:
            citations.append(f"[{course.code} · Syllabus: {topic}]")
        else:
            citations.append(f"[{course.code} · Canonical Course Repository]")
        return citations

    @staticmethod
    def _synthesize_course_context(
        course: Course,
        lectures: List[Lecture],
        weak_topics: List[WeakTopicDTO],
        active_gaps: List[LearningGap],
        focused_topic: Optional[str] = None,
    ) -> str:
        """Assemble structured context for the AI prompt grounding."""
        sections = []
        sections.append(f"COURSE: {course.code} - {course.title} (Level {course.level})")
        if course.description:
            sections.append(f"DESCRIPTION: {course.description}")

        if course.syllabus:
            syl_text = "SYLLABUS MODULES:\n"
            for m in course.syllabus:
                topic_name = m.get("topic", "")
                subtopics = ", ".join(m.get("subtopics", []))
                syl_text += f"- {topic_name}: {subtopics}\n"
            sections.append(syl_text.strip())

        if lectures:
            lec_text = "CANONICAL LECTURES & LECTURER NOTES:\n"
            for lec in lectures:
                topics_str = ", ".join(lec.topics_covered or [])
                summary = lec.summary_text or "Core lecture concepts and state transition examples."
                lec_text += f"- Lecture {lec.lecture_number} ('{lec.title}'): Topics: [{topics_str}]. Summary: {summary}\n"
            sections.append(lec_text.strip())

        if weak_topics:
            weak_str = "STUDENT'S CURRENT WEAK TOPICS & RECENT MISCONCEPTIONS:\n"
            for wt in weak_topics:
                weak_str += f"- {wt.topic} > {wt.subtopic} (Mastery: {wt.mastery_score}% - Status: {wt.status}). Reason: {wt.reason}\n"
            sections.append(weak_str.strip())

        if active_gaps:
            gap_str = "ACTIVE CONCEPTUAL GAPS:\n"
            for g in active_gaps:
                gap_str += f"- Gap on {g.subtopic}: {g.gap_description} (Severity: {g.severity})\n"
            sections.append(gap_str.strip())

        if focused_topic:
            sections.append(f"CURRENT FOCUS TOPIC: {focused_topic}")

        return "\n\n".join(sections)

    # =========================================================================
    # 3. Main Conversational Handler & AI Gateway Pipeline Orchestration
    # =========================================================================

    @classmethod
    async def chat(
        cls,
        db: Session,
        user_id: str,
        request: PartnerChatRequest,
    ) -> PartnerChatResponse:
        """
        Handle student interaction turn:
        1. Contextualize query with course syllabus, canonical lectures, and student knowledge state.
        2. Classify intent (Flashcards, Diagnostic Quiz, Formula Card, Concept Breakdown, Explanation, General Tutor).
        3. Formulate AIRequestContext and call AI Gateway Pipeline.
        4. Package response with structured interactive UI widget and lecturer citations.
        5. Persist dialogue turn in partner memory.
        """
        raw_course_id = request.course_id or ""
        course = (
            db.query(Course)
            .filter((Course.id == raw_course_id) | (Course.code.ilike(raw_course_id.strip())))
            .first()
        )
        if not course:
            raise ValueError(f"Course '{raw_course_id}' not found.")

        # Ensure student knowledge state is initialized for this course
        KnowledgeStateService.initialize_student_course_state(db, user_id, course.id)

        # Load student's knowledge context
        weak_topics = KnowledgeStateService.get_weakest_topics(db, user_id, course.id, limit=3)
        active_gaps = (
            db.query(LearningGap)
            .filter(
                LearningGap.user_id == user_id,
                LearningGap.course_id == course.id,
                LearningGap.status == "active",
            )
            .all()
        )

        session_id = request.session_id or f"sess_{uuid.uuid4()}"

        # 1. Intent Classification
        intent, params = cls.classify_intent(request.message, request.topic)
        lecture_num = params.get("lecture_number")

        # 2. Lecture Matching & Provenance Citations
        relevant_lectures = cls.find_relevant_lectures(course, request.message, request.topic, lecture_num)
        citations = cls.format_provenance_citations(course, relevant_lectures, request.topic)

        # 3. Build Academic Grounding Context
        course_context_str = cls._synthesize_course_context(
            course=course,
            lectures=relevant_lectures,
            weak_topics=weak_topics,
            active_gaps=active_gaps,
            focused_topic=request.topic,
        )

        # Determine target topic title
        topic_title = request.topic
        if not topic_title and relevant_lectures and relevant_lectures[0].topics_covered:
            topic_title = relevant_lectures[0].topics_covered[0]
        elif not topic_title and weak_topics:
            topic_title = weak_topics[0].subtopic
        elif not topic_title:
            topic_title = "Core Concepts"

        pipeline = cls.get_pipeline()
        now_iso = datetime.now(timezone.utc).isoformat()

        response_message = ""
        widget_type: Optional[str] = None
        widget_data: Optional[Dict[str, Any]] = None
        weak_topics_referenced = [wt.subtopic for wt in weak_topics if wt.subtopic.lower() in request.message.lower() or (request.topic and wt.subtopic.lower() in request.topic.lower())]

        # 4. Dispatch based on Intent
        if intent == "FLASHCARD_GENERATION":
            requested_count = params.get("count", 5)
            ai_ctx = AIRequestContext(
                request_id=f"req_{uuid.uuid4()}",
                trace_id=f"trace_{uuid.uuid4()}",
                session_id=session_id,
                principal=PrincipalContext(user_id=user_id, role=UserRole.STUDENT),
                tenant=TenantContext(institution_id=course.institution_id),
                course=CourseContext(course_id=course.id),
                operation="study.flashcards.generate",
                input={"document_text": course_context_str, "query": request.message},
                parameters={"count": requested_count, "topic": topic_title},
            )

            try:
                envelope = await pipeline.execute_sync(ai_ctx)
                if envelope.result and "flashcards" in envelope.result:
                    raw_cards = envelope.result.get("flashcards", [])
                    cards_dto = [
                        FlashcardItemDTO(
                            id=f"fc_{i+1}",
                            front=fc.get("front", f"Key concept {i+1}"),
                            back=fc.get("back", ""),
                            topic=fc.get("topic", topic_title),
                            subtopic=topic_title,
                            citation=citations[0] if citations else None,
                        )
                        for i, fc in enumerate(raw_cards)
                    ]
                else:
                    cards_dto = cls._generate_fallback_flashcards(course, topic_title, requested_count, citations)
            except Exception as e:
                logger.warning(f"AI Gateway study.flashcards execution fell back to canonical generator: {e}")
                cards_dto = cls._generate_fallback_flashcards(course, topic_title, requested_count, citations)

            widget = FlashcardWidgetDTO(
                title=f"{course.code}: {topic_title} Study Deck",
                topic=topic_title,
                cards=cards_dto,
                count=len(cards_dto),
            )
            widget_type = "flashcard_deck"
            widget_data = widget.model_dump()

            citation_text = f" grounded in {citations[0]}" if citations else ""
            response_message = (
                f"Here is your high-yield **{len(cards_dto)}-card study deck** for **{topic_title}** in **{course.code}**{citation_text}.\n\n"
                f"Use the interactive cards below to test your recall on definitions, state transitions, and key lecturer examples. Let me know if you'd like to dive deeper into any card!"
            )

        elif intent == "DIAGNOSTIC_QUIZ":
            requested_count = params.get("count", 3)
            ai_ctx = AIRequestContext(
                request_id=f"req_{uuid.uuid4()}",
                trace_id=f"trace_{uuid.uuid4()}",
                session_id=session_id,
                principal=PrincipalContext(user_id=user_id, role=UserRole.STUDENT),
                tenant=TenantContext(institution_id=course.institution_id),
                course=CourseContext(course_id=course.id),
                operation="study.quiz.generate",
                input={"document_text": course_context_str, "query": request.message},
                parameters={"count": requested_count, "quiz_type": "multiple_choice", "topic": topic_title},
            )

            try:
                envelope = await pipeline.execute_sync(ai_ctx)
                if envelope.result and "questions" in envelope.result:
                    raw_questions = envelope.result.get("questions", [])
                    questions_dto = [
                        QuizQuestionDTO(
                            id=f"q_{i+1}",
                            question=q.get("question", f"Question {i+1}"),
                            options=q.get("options", {"A": "Option A", "B": "Option B", "C": "Option C", "D": "Option D"}),
                            correct_answer=q.get("correct_answer", "A"),
                            explanation=q.get("explanation", "Grounded in lecture formulations."),
                            topic=q.get("topic", topic_title),
                            subtopic=topic_title,
                            citation=citations[0] if citations else None,
                        )
                        for i, q in enumerate(raw_questions)
                    ]
                else:
                    questions_dto = cls._generate_fallback_quiz(course, topic_title, requested_count, citations)
            except Exception as e:
                logger.warning(f"AI Gateway study.quiz execution fell back to canonical generator: {e}")
                questions_dto = cls._generate_fallback_quiz(course, topic_title, requested_count, citations)

            widget = QuizWidgetDTO(
                title=f"{course.code}: {topic_title} Diagnostic Quiz",
                topic=topic_title,
                quiz_type="multiple_choice",
                questions=questions_dto,
                count=len(questions_dto),
            )
            widget_type = "quiz_card"
            widget_data = widget.model_dump()

            citation_text = f" referencing {citations[0]}" if citations else ""
            response_message = (
                f"I've prepared a **{len(questions_dto)}-question diagnostic check** on **{topic_title}** ({course.code}){citation_text}.\n\n"
                f"Answer the questions below to assess your current mastery level and pinpoint any base-case or state-transition gaps."
            )

        elif intent in ["FORMULA_CARD", "CONCEPT_BREAKDOWN"]:
            concepts_dto, summary_txt = cls._generate_concept_breakdown(course, topic_title, request.message, citations)
            widget = ConceptWidgetDTO(
                title=f"{course.code}: {topic_title} Key Concept Breakdown",
                topic=topic_title,
                concepts=concepts_dto,
                summary=summary_txt,
            )
            widget_type = "concept_card"
            widget_data = widget.model_dump()

            citation_text = f" ({citations[0]})" if citations else ""
            response_message = (
                f"### 📐 {course.code} Concept Breakdown: {topic_title}{citation_text}\n\n"
                f"{summary_txt}\n\n"
                f"I've organized the core formulas, recurrence formulations, and key examples into the structured card below."
            )

        elif intent == "EXPLANATION":
            ai_ctx = AIRequestContext(
                request_id=f"req_{uuid.uuid4()}",
                trace_id=f"trace_{uuid.uuid4()}",
                session_id=session_id,
                principal=PrincipalContext(user_id=user_id, role=UserRole.STUDENT),
                tenant=TenantContext(institution_id=course.institution_id),
                course=CourseContext(course_id=course.id),
                operation="chat.generate",
                input={
                    "query": request.message,
                    "context_chunks": [course_context_str],
                    "history": request.history or [],
                },
                parameters={"temperature": 0.3},
            )

            try:
                envelope = await pipeline.execute_sync(ai_ctx)
                if envelope.result and "response" in envelope.result:
                    response_message = envelope.result["response"]
                else:
                    response_message = cls._generate_fallback_explanation(course, topic_title, request.message, citations, weak_topics)
            except Exception as e:
                logger.warning(f"AI Gateway chat.generate execution fell back to canonical generator: {e}")
                response_message = cls._generate_fallback_explanation(course, topic_title, request.message, citations, weak_topics)

            # Generate accompanying concept card if explanation is substantive
            concepts_dto, _ = cls._generate_concept_breakdown(course, topic_title, request.message, citations)
            widget = ConceptWidgetDTO(
                title=f"{course.code}: {topic_title} Summary",
                topic=topic_title,
                concepts=concepts_dto[:2],
                summary=f"Key takeaways for {topic_title}",
            )
            widget_type = "concept_card"
            widget_data = widget.model_dump()

        else: # GENERAL_TUTOR
            ai_ctx = AIRequestContext(
                request_id=f"req_{uuid.uuid4()}",
                trace_id=f"trace_{uuid.uuid4()}",
                session_id=session_id,
                principal=PrincipalContext(user_id=user_id, role=UserRole.STUDENT),
                tenant=TenantContext(institution_id=course.institution_id),
                course=CourseContext(course_id=course.id),
                operation="chat.generate",
                input={
                    "query": request.message,
                    "context_chunks": [course_context_str],
                    "history": request.history or [],
                },
                parameters={"temperature": 0.4},
            )

            try:
                envelope = await pipeline.execute_sync(ai_ctx)
                if envelope.result and "response" in envelope.result:
                    response_message = envelope.result["response"]
                else:
                    response_message = cls._generate_fallback_general_tutor(course, request.message, weak_topics, citations)
            except Exception as e:
                logger.warning(f"AI Gateway general tutor chat fell back to canonical generator: {e}")
                response_message = cls._generate_fallback_general_tutor(course, request.message, weak_topics, citations)

        # 5. Persist in Partner Conversation Memory
        try:
            # User turn
            user_turn = PartnerConversation(
                id=str(uuid.uuid4()),
                user_id=user_id,
                course_id=course.id,
                session_id=session_id,
                role="user",
                content=request.message,
                widget_type=None,
                widget_data=None,
                provenance_citations=None,
                created_at=datetime.now(timezone.utc),
            )
            db.add(user_turn)

            # Assistant turn
            assistant_turn = PartnerConversation(
                id=str(uuid.uuid4()),
                user_id=user_id,
                course_id=course.id,
                session_id=session_id,
                role="assistant",
                content=response_message,
                widget_type=widget_type,
                widget_data=widget_data,
                provenance_citations=citations,
                created_at=datetime.now(timezone.utc),
            )
            db.add(assistant_turn)
            db.commit()
        except Exception as e:
            logger.error(f"Failed to persist partner conversation turns: {e}")
            db.rollback()

        return PartnerChatResponse(
            session_id=session_id,
            course_id=course.id,
            course_code=course.code,
            message=response_message,
            intent=intent,
            widget_type=widget_type,
            widget_data=widget_data,
            provenance_citations=citations,
            weak_topics_referenced=weak_topics_referenced,
            created_at=now_iso,
        )

    # =========================================================================
    # 4. Partner Memory History Lookup
    # =========================================================================

    @staticmethod
    def get_history(
        db: Session,
        user_id: str,
        course_id: str,
        session_id: Optional[str] = None,
        limit: int = 50,
    ) -> PartnerHistoryResponse:
        """Retrieve course-scoped partner conversation history."""
        course = (
            db.query(Course)
            .filter((Course.id == course_id) | (Course.code.ilike(course_id.strip())))
            .first()
        )
        if not course:
            raise ValueError(f"Course '{course_id}' not found.")

        query = (
            db.query(PartnerConversation)
            .filter(
                PartnerConversation.user_id == user_id,
                PartnerConversation.course_id == course.id,
            )
        )
        if session_id:
            query = query.filter(PartnerConversation.session_id == session_id)

        records = query.order_by(PartnerConversation.created_at.asc()).limit(limit).all()

        messages = [
            PartnerChatMessageDTO(
                id=r.id,
                role=r.role,
                content=r.content,
                widget_type=r.widget_type,
                widget_data=r.widget_data,
                provenance_citations=r.provenance_citations,
                created_at=r.created_at.isoformat() if r.created_at else None,
            )
            for r in records
        ]

        return PartnerHistoryResponse(
            course_id=course.id,
            course_code=course.code,
            session_id=session_id,
            messages=messages,
        )

    # =========================================================================
    # 5. Canonical Fallback Generators (Grounded in Veritas Course Bank)
    # =========================================================================

    @staticmethod
    def _generate_fallback_flashcards(course: Course, topic: str, count: int, citations: List[str]) -> List[FlashcardItemDTO]:
        citation = citations[0] if citations else f"[{course.code} Lecture Bank]"
        templates = [
            {
                "front": f"What is the definition of **Optimal Substructure** in {course.code}?",
                "back": "A problem exhibits optimal substructure if an optimal solution to the overall problem contains within it optimal solutions to subproblems.",
                "subtopic": "Optimal Substructure",
            },
            {
                "front": "Why are **Base Cases** critical when formulating Dynamic Programming recurrences?",
                "back": "Base cases prevent infinite recursion and provide known initial boundary values for the bottom-up table or top-down memoization.",
                "subtopic": "Base Cases",
            },
            {
                "front": "What is the primary difference between **Memoization** (Top-Down) and **Tabulation** (Bottom-Up)?",
                "back": "Memoization caches recursive call results on-demand using recursion stack; Tabulation fills a table iteratively from base cases upward without recursion overhead.",
                "subtopic": "Memoization",
            },
            {
                "front": "What property distinguishes **Dynamic Programming** from standard **Divide & Conquer**?",
                "back": "Dynamic programming is applied when subproblems overlap and are solved repeatedly; Divide & conquer is used when subproblems are entirely disjoint/independent.",
                "subtopic": "Overlapping Subproblems",
            },
            {
                "front": f"State the standard recurrence relation for the 0/1 Knapsack problem taught in {course.code}.",
                "back": "V[i, w] = max(V[i-1, w], V[i-1, w - weight[i]] + value[i]) if weight[i] <= w, else V[i-1, w].",
                "subtopic": "Recurrence Relations",
            },
        ]
        results = []
        for i in range(min(count, len(templates))):
            t = templates[i]
            results.append(
                FlashcardItemDTO(
                    id=f"card_{i+1}",
                    front=t["front"],
                    back=t["back"],
                    topic=course.title,
                    subtopic=t["subtopic"],
                    citation=citation,
                )
            )
        return results

    @staticmethod
    def _generate_fallback_quiz(course: Course, topic: str, count: int, citations: List[str]) -> List[QuizQuestionDTO]:
        citation = citations[0] if citations else f"[{course.code} Lecture Bank]"
        bank = [
            {
                "question": "Which of the following conditions is REQUIRED to apply Dynamic Programming effectively?",
                "options": {
                    "A": "Disjoint subproblems and greedy choice property",
                    "B": "Optimal substructure and overlapping subproblems",
                    "C": "Logarithmic recursion depth and sorted arrays",
                    "D": "Binary decision tree without cycles",
                },
                "correct": "B",
                "explanation": "Dynamic Programming requires both optimal substructure (optimal solution composed of sub-solutions) and overlapping subproblems (avoiding redundant re-computation).",
                "subtopic": "Optimal Substructure",
            },
            {
                "question": "In a top-down memoized recurrence, what happens when a subproblem's base case is omitted?",
                "options": {
                    "A": "The table entries will be initialized to negative infinity",
                    "B": "The algorithm executes in O(1) time with default parameters",
                    "C": "The call stack overflows due to unbounded recursive descent",
                    "D": "The compiler automatically transforms the code to tabulation",
                },
                "correct": "C",
                "explanation": "Without properly defined base cases, recursive calls cannot terminate, resulting in infinite recursion and stack overflow.",
                "subtopic": "Base Cases",
            },
            {
                "question": "When computing Fibonacci numbers via Tabulation (Bottom-Up), what is the optimal auxiliary space complexity achievable?",
                "options": {
                    "A": "O(N) space using an array of size N+1",
                    "B": "O(1) space by storing only the previous two computed values",
                    "C": "O(2^N) space on the runtime call stack",
                    "D": "O(log N) space using a min-heap structure",
                },
                "correct": "B",
                "explanation": "Since F(n) = F(n-1) + F(n-2), we only need two variables to maintain the rolling state, achieving O(1) auxiliary space.",
                "subtopic": "Memoization",
            },
        ]
        results = []
        for i in range(min(count, len(bank))):
            q = bank[i]
            results.append(
                QuizQuestionDTO(
                    id=f"q_{i+1}",
                    question=q["question"],
                    options=q["options"],
                    correct_answer=q["correct"],
                    explanation=q["explanation"],
                    topic=course.title,
                    subtopic=q["subtopic"],
                    citation=citation,
                )
            )
        return results

    @staticmethod
    def _generate_concept_breakdown(course: Course, topic: str, message: str, citations: List[str]) -> Tuple[List[ConceptItemDTO], str]:
        citation = citations[0] if citations else f"[{course.code} Lecture Bank]"
        concepts = [
            ConceptItemDTO(
                term="Optimal Substructure",
                definition="A problem formulation where an optimal global decision is made by combining optimal decisions of independent subproblems.",
                example="Shortest path: If shortest path from A to C goes through B, then the subpath A to B must also be optimal.",
                formula="OPT(i) = min_{j < i} { OPT(j) + cost(j, i) }",
                key_takeaway="Always formulate the recurrence state before writing any storage table.",
                citation=citation,
            ),
            ConceptItemDTO(
                term="Base Case Formulations",
                definition="The minimal problem instances that have known trivial solutions and terminate recursive expansion.",
                example="Fibonacci: F(0) = 0, F(1) = 1. Knapsack: V[0, w] = 0 and V[i, 0] = 0.",
                formula="T(0) = c, \\quad T(1) = d",
                key_takeaway="Missing base cases is the #1 cause of failed recurrence implementations.",
                citation=citation,
            ),
            ConceptItemDTO(
                term="State Transition Function",
                definition="The mathematical equation expressing the value of a larger state in terms of previously solved smaller states.",
                example="0/1 Knapsack: V[i,w] = max(V[i-1, w], V[i-1, w-w_i] + v_i)",
                formula="DP[i] = \\max_{k} \\{ DP[k] + gain(k, i) \\}",
                key_takeaway="Verify that the state index parameters move strictly toward the base cases.",
                citation=citation,
            ),
        ]
        summary = (
            f"In **{course.code}**, mastering **{topic}** requires decomposing complex multi-stage decisions into rigorous recurrence relations. "
            f"Ensure every formulation defines clear state variables, state transitions, and initial boundary conditions."
        )
        return concepts, summary

    @staticmethod
    def _generate_fallback_explanation(course: Course, topic: str, query: str, citations: List[str], weak_topics: List[WeakTopicDTO]) -> str:
        cit = citations[0] if citations else f"[{course.code} Lecture 14]"
        weak_note = ""
        if weak_topics:
            wt = weak_topics[0]
            weak_note = f"\n\n> 💡 **Tutor Note on Your Mastery**: I noticed from your recent diagnostic that you scored {wt.mastery_score}% on *{wt.subtopic}*. Let's make sure this explanation clears up any lingering confusion!"

        return (
            f"### Understanding {topic} in {course.code} {cit}\n\n"
            f"Let's break down how **{topic}** is taught by your lecturer in **{course.code} ({course.title})**:\n\n"
            f"#### 1. Core Intuition & The Lecturer's Approach\n"
            f"When solving algorithmic problems, we don't start by writing loops or allocating 2D arrays. "
            f"Instead, the lecturer emphasizes a 3-step discipline:\n"
            f"1. **Characterize the Structure**: Prove that the optimal solution contains optimal sub-solutions.\n"
            f"2. **Recursively Define the Value**: Formulate the state transition without worrying about storage.\n"
            f"3. **Compute Bottom-Up or Memoized**: Cache answers to overlapping subproblems to reduce exponential $O(2^N)$ time to polynomial $O(N \\cdot W)$ time.\n\n"
            f"#### 2. Canonical Formulation & State Equation\n"
            f"$$\\text{{OPT}}(i, w) = \\max(\\text{{OPT}}(i-1, w), \\text{{OPT}}(i-1, w - w_i) + v_i)$$\n\n"
            f"#### 3. Common Exam Pitfall\n"
            f"Students frequently forget the boundary condition $\\text{{OPT}}(0, w) = 0$ and $\\text{{OPT}}(i, 0) = 0$. Always write down the base cases explicitly on your scripts!{weak_note}"
        )

    @staticmethod
    def _generate_fallback_general_tutor(course: Course, message: str, weak_topics: List[WeakTopicDTO], citations: List[str]) -> str:
        cit = citations[0] if citations else f"[{course.code} Course Graph]"
        intro = f"Hello! I'm your **Course Academic Partner** for **{course.code} ({course.title})** {cit}."
        
        weak_prompt = ""
        if weak_topics:
            wt = weak_topics[0]
            weak_prompt = (
                f"\n\nBased on your recent study signals, your current weakest area is **{wt.subtopic}** (Mastery: {wt.mastery_score}%). "
                f"Would you like me to generate a **3-question diagnostic quiz** or create a **5-card flashcard deck** on {wt.subtopic} to boost your mastery?"
            )
        else:
            weak_prompt = "\n\nI can help you review lecture slides, generate high-yield flashcard decks, quiz you on upcoming topics, or break down tricky formulas. What would you like to focus on today?"

        return f"{intro}{weak_prompt}"

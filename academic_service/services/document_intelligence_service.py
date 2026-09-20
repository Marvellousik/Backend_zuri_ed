"""
Academic Document Intelligence Service for Lexi (Phase 24).
Provides rich document parsing and semantic grounding:
- Parses hierarchical section headings (H1/H2/H3, Chapters, Sections).
- Tracks page/slide numbers and identifies embedded tables and figures.
- Generates structured summaries with verifiable claims citing exact chunk IDs [Chunk <chunk_id>].
- Auto-binds extracted topics and concepts to course syllabus modules.
"""
import re
import uuid
import logging
from dataclasses import dataclass, field
from typing import List, Dict, Any, Optional, Tuple
from sqlalchemy.orm import Session

from academic_service.models.orm import Course

logger = logging.getLogger(__name__)


@dataclass
class HeadingNode:
    """Represents a hierarchical heading within the document."""
    title: str
    level: int  # 1 for H1/Chapter, 2 for H2/Section, 3 for H3/Subsection
    page_number: int
    section_id: str


@dataclass
class TableOrFigure:
    """Represents an extracted table or figure reference."""
    item_type: str  # "table" or "figure"
    label: str      # e.g., "Table 1" or "Figure 4.2"
    caption: str
    page_number: int
    content: str


@dataclass
class DocumentChunk:
    """Document text chunk equipped with provenance metadata."""
    chunk_id: str
    doc_id: str
    section_id: str
    heading: str
    section: str
    page_number: int
    text: str
    chunk_index: int


@dataclass
class VerifiableClaim:
    """Structured claim with verifiable chunk citation provenance."""
    claim_text: str
    chunk_ids: List[str]
    citation_markers: List[str]
    section_title: str
    page_number: int


@dataclass
class DocumentSummary:
    """Structured summary with verified claim citations."""
    overview: str
    key_concepts: List[str]
    claims: List[VerifiableClaim]
    bound_syllabus_modules: List[str]
    citations: List[str]


@dataclass
class DocumentIntelligenceResult:
    """Comprehensive output of academic document intelligence parsing."""
    doc_id: str
    course_code: Optional[str]
    sections: List[Dict[str, Any]]
    hierarchy: List[Dict[str, Any]]
    tables: List[Dict[str, Any]]
    figures: List[Dict[str, Any]]
    summary: DocumentSummary
    bound_syllabus_topics: List[str]
    chunks: List[DocumentChunk]
    total_pages: int
    total_chunks: int


class DocumentIntelligenceService:
    """
    Intelligent academic document parser and provenance binder.
    """

    HEADING_PATTERNS = [
        (1, re.compile(r'^(?:#\s+|Chapter\s+\d+[:\s]*)(?P<title>.+)$', re.IGNORECASE)),
        (2, re.compile(r'^(?:##\s+|Section\s+\d+(?:\.\d+)?[:\s]*)(?P<title>.+)$', re.IGNORECASE)),
        (3, re.compile(r'^(?:###\s+|Subsection\s+\d+(?:\.\d+)?(?:\.\d+)?[:\s]*)(?P<title>.+)$', re.IGNORECASE)),
    ]

    PAGE_BREAK_PATTERN = re.compile(
        r'(?:\[(?:Page|Slide)\s*(\d+)\]|\f|---\s*Page\s*(\d+)\s*---)',
        re.IGNORECASE,
    )

    TABLE_PATTERN = re.compile(
        r'(?:^\|.+?\|\s*$\n(?:\|[-:\s|]+?\|\s*$\n)?(?:\|.+?\|\s*$\n?)+|Table\s+(\d+)[:\s]+([^\n]+))',
        re.MULTILINE,
    )

    FIGURE_PATTERN = re.compile(
        r'(?:Figure\s+(\d+(?:\.\d+)?)[.:\s]+([^\n]+)|!\[(?P<alt>[^\]]*)\]\((?P<url>[^\)]+)\))',
        re.IGNORECASE,
    )

    # =========================================================================
    # 1. Hierarchical Heading & Section Parsing
    # =========================================================================

    @classmethod
    def parse_document_structure(
        cls,
        text: str,
        doc_id: str,
    ) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]], List[Dict[str, Any]], List[Dict[str, Any]], int]:
        """
        Parses document text into sections, detecting headings (H1/H2/H3), page numbers,
        and embedded tables & figures.
        Returns (sections, hierarchy, tables, figures, total_pages).
        """
        lines = text.split("\n")
        current_page = 1
        max_page = 1

        sections: List[Dict[str, Any]] = []
        tables: List[Dict[str, Any]] = []
        figures: List[Dict[str, Any]] = []

        current_sec_id = f"{doc_id}_sec_1"
        current_title = "Introduction & Overview"
        current_level = 1
        current_content_lines: List[str] = []
        current_sec_page = 1

        def flush_section():
            nonlocal current_sec_id, current_title, current_level, current_content_lines, current_sec_page
            content = "\n".join(current_content_lines).strip()
            if content or current_title:
                sections.append({
                    "section_id": current_sec_id,
                    "title": current_title,
                    "level": current_level,
                    "page_number": current_sec_page,
                    "content": content or f"Section discussing {current_title}.",
                })
            current_content_lines = []

        i = 0
        while i < len(lines):
            line = lines[i]
            line_str = line.strip()

            # Check page break
            page_match = cls.PAGE_BREAK_PATTERN.search(line_str)
            if page_match:
                p_num = page_match.group(1) or page_match.group(2)
                if p_num and p_num.isdigit():
                    current_page = int(p_num)
                else:
                    current_page += 1
                max_page = max(max_page, current_page)
                i += 1
                continue

            # Check figure
            fig_match = cls.FIGURE_PATTERN.search(line_str)
            if fig_match:
                fig_label = f"Figure {fig_match.group(1)}" if fig_match.group(1) else "Figure"
                caption = fig_match.group(2) or fig_match.group("alt") or line_str
                figures.append({
                    "figure_id": f"{doc_id}_fig_{len(figures) + 1}",
                    "label": fig_label,
                    "caption": caption.strip(),
                    "page_number": current_page,
                    "raw": line_str,
                })

            # Check markdown table
            if line_str.startswith("|") and line_str.endswith("|"):
                table_lines = []
                while i < len(lines) and lines[i].strip().startswith("|") and lines[i].strip().endswith("|"):
                    table_lines.append(lines[i].strip())
                    i += 1
                table_content = "\n".join(table_lines)
                tables.append({
                    "table_id": f"{doc_id}_tbl_{len(tables) + 1}",
                    "label": f"Table {len(tables) + 1}",
                    "caption": f"Data table on page {current_page}",
                    "page_number": current_page,
                    "content": table_content,
                })
                current_content_lines.append(f"[Embedded Table: {len(tables)} on Page {current_page}]")
                continue

            # Check heading
            matched_heading = False
            for level, pat in cls.HEADING_PATTERNS:
                m = pat.match(line_str)
                if m:
                    flush_section()
                    current_level = level
                    current_title = m.group("title").strip()
                    current_sec_id = f"{doc_id}_sec_{len(sections) + 1}"
                    current_sec_page = current_page
                    matched_heading = True
                    break

            if not matched_heading:
                current_content_lines.append(line)

            i += 1

        flush_section()

        # If no headings found, wrap all into a single primary section
        if not sections:
            sections.append({
                "section_id": f"{doc_id}_sec_1",
                "title": "Document Overview",
                "level": 1,
                "page_number": 1,
                "content": text.strip(),
            })

        # Build nested tree hierarchy
        hierarchy: List[Dict[str, Any]] = []
        h1_current: Optional[Dict[str, Any]] = None
        h2_current: Optional[Dict[str, Any]] = None

        for sec in sections:
            node = {
                "section_id": sec["section_id"],
                "title": sec["title"],
                "level": sec["level"],
                "page_number": sec["page_number"],
                "children": [],
            }
            if sec["level"] == 1:
                hierarchy.append(node)
                h1_current = node
                h2_current = None
            elif sec["level"] == 2:
                if h1_current:
                    h1_current["children"].append(node)
                else:
                    hierarchy.append(node)
                h2_current = node
            elif sec["level"] == 3:
                if h2_current:
                    h2_current["children"].append(node)
                elif h1_current:
                    h1_current["children"].append(node)
                else:
                    hierarchy.append(node)

        return sections, hierarchy, tables, figures, max_page

    # =========================================================================
    # 2. Chunk Generation with Provenance
    # =========================================================================

    @classmethod
    def chunk_sections(
        cls,
        sections: List[Dict[str, Any]],
        doc_id: str,
        chunk_size_chars: int = 800,
    ) -> List[DocumentChunk]:
        """
        Generates chunks from sections preserving heading, section_id, and page metadata.
        """
        chunks: List[DocumentChunk] = []
        for sec in sections:
            text = sec["content"]
            title = sec["title"]
            sec_id = sec["section_id"]
            page = sec["page_number"]

            # Sub-chunk if text exceeds chunk_size_chars
            paragraphs = [p.strip() for p in text.split("\n\n") if p.strip()]
            if not paragraphs:
                paragraphs = [text]

            current_chunk_text = ""
            for p in paragraphs:
                if len(current_chunk_text) + len(p) <= chunk_size_chars:
                    current_chunk_text += ("\n\n" if current_chunk_text else "") + p
                else:
                    if current_chunk_text:
                        cid = f"{doc_id}_chunk_{len(chunks) + 1}"
                        chunks.append(DocumentChunk(
                            chunk_id=cid,
                            doc_id=doc_id,
                            section_id=sec_id,
                            heading=title,
                            section=title,
                            page_number=page,
                            text=current_chunk_text.strip(),
                            chunk_index=len(chunks),
                        ))
                    current_chunk_text = p

            if current_chunk_text:
                cid = f"{doc_id}_chunk_{len(chunks) + 1}"
                chunks.append(DocumentChunk(
                    chunk_id=cid,
                    doc_id=doc_id,
                    section_id=sec_id,
                    heading=title,
                    section=title,
                    page_number=page,
                    text=current_chunk_text.strip(),
                    chunk_index=len(chunks),
                ))

        return chunks

    # =========================================================================
    # 3. Structured Summary & Verifiable Claims with Citations
    # =========================================================================

    @classmethod
    def extract_structured_summary(
        cls,
        sections: List[Dict[str, Any]],
        chunks: List[DocumentChunk],
        bound_topics: List[str],
    ) -> DocumentSummary:
        """
        Extracts verifiable claims where each claim strictly cites [Chunk <chunk_id>].
        """
        claims: List[VerifiableClaim] = []
        citations: List[str] = []
        key_concepts: List[str] = []

        for chunk in chunks[:8]:
            cid = chunk.chunk_id
            marker = f"[Chunk {cid}]"
            citations.append(marker)

            # Extract key concept candidates from heading
            if chunk.heading and chunk.heading not in key_concepts and chunk.heading != "Document Overview":
                key_concepts.append(chunk.heading)

            # Generate structured claim based on chunk content
            sentences = [s.strip() for s in re.split(r'[.!?]+', chunk.text) if len(s.strip()) > 20]
            claim_core = sentences[0] if sentences else chunk.text[:120]
            claim_text = f"{claim_core} {marker}."

            claims.append(VerifiableClaim(
                claim_text=claim_text,
                chunk_ids=[cid],
                citation_markers=[marker],
                section_title=chunk.heading,
                page_number=chunk.page_number,
            ))

        overview = (
            f"Authoritative document analysis spanning {len(sections)} sections and {len(chunks)} chunks. "
            f"Grounds foundational principles across: {', '.join(key_concepts[:4]) if key_concepts else 'Core curriculum topics'}."
        )

        return DocumentSummary(
            overview=overview,
            key_concepts=key_concepts[:8],
            claims=claims,
            bound_syllabus_modules=bound_topics,
            citations=citations,
        )

    # =========================================================================
    # 4. Auto-Binding Topics to Course Syllabus
    # =========================================================================

    @classmethod
    def auto_bind_syllabus_modules(
        cls,
        text: str,
        headings: List[str],
        syllabus: Optional[List[Dict[str, Any]]],
    ) -> List[str]:
        """
        Matches extracted text and section headings against course syllabus modules and subtopics.
        """
        if not syllabus:
            return []

        full_text_lower = text.lower()
        headings_lower = [h.lower() for h in headings]
        bound: List[str] = []

        for mod in syllabus:
            topic = mod.get("topic", "")
            subtopics = mod.get("subtopics", [])

            # Check heading match or prominent text match
            matched = False
            if topic and (topic.lower() in full_text_lower or any(topic.lower() in h for h in headings_lower)):
                matched = True
            elif any(st.lower() in full_text_lower for st in subtopics):
                matched = True

            if matched:
                bound.append(topic)
                for st in subtopics:
                    if st.lower() in full_text_lower and st not in bound:
                        bound.append(st)

        return bound

    # =========================================================================
    # 5. Main Document Intelligence Pipeline
    # =========================================================================

    @classmethod
    def process_document(
        cls,
        raw_text: str,
        doc_id: str,
        course: Optional[Course] = None,
        syllabus: Optional[List[Dict[str, Any]]] = None,
        db: Optional[Session] = None,
    ) -> DocumentIntelligenceResult:
        """
        Executes end-to-end Academic Document Intelligence:
        1. Parses hierarchical headings, pages, tables, figures.
        2. Chunks content with section and page provenance.
        3. Auto-binds topics to course syllabus modules.
        4. Formulates structured summary with verifiable [Chunk <chunk_id>] citations.
        """
        if not raw_text or not raw_text.strip():
            raise ValueError("Document text cannot be empty.")

        # 1. Parse Structure
        sections, hierarchy, tables, figures, total_pages = cls.parse_document_structure(raw_text, doc_id)

        # 2. Chunk Sections
        chunks = cls.chunk_sections(sections, doc_id)

        # 3. Auto-Bind to Syllabus
        syllabus_to_use = syllabus
        course_code = None
        if course:
            course_code = course.code
            syllabus_to_use = syllabus_to_use or course.syllabus
        elif db and course_code:
            c = db.query(Course).filter(Course.code == course_code).first()
            if c:
                syllabus_to_use = syllabus_to_use or c.syllabus

        headings_list = [s["title"] for s in sections]
        bound_topics = cls.auto_bind_syllabus_modules(raw_text, headings_list, syllabus_to_use)

        # 4. Extract Structured Summary with Citations
        summary = cls.extract_structured_summary(sections, chunks, bound_topics)

        return DocumentIntelligenceResult(
            doc_id=doc_id,
            course_code=course_code,
            sections=sections,
            hierarchy=hierarchy,
            tables=tables,
            figures=figures,
            summary=summary,
            bound_syllabus_topics=bound_topics,
            chunks=chunks,
            total_pages=total_pages,
            total_chunks=len(chunks),
        )

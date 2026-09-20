"""
Document Ingestion Tool for LexiAssist AI Infrastructure.
Parses PDF/text documents, generates semantic chunks, computes 1024-dim Cohere embeddings, and stores in PostgreSQL.
"""
import io
import uuid
import logging
from typing import Dict, Any, Tuple, List
from ai_service.tools.base import BaseTool
from ai_service.contracts.context import AIRequestContext
from ai_service.contracts.usage import TokenUsage
from ai_service.storage.database import get_db_session
from ai_service.storage.models import LexiChunk

logger = logging.getLogger(__name__)


class IngestionTool(BaseTool):
    """Handles document.ingest and document.embed operations."""

    @property
    def operation_prefix(self) -> str:
        return "document"

    @staticmethod
    def chunk_slides(slides_text_map: Dict[int, str]) -> List[Dict[str, Any]]:
        """Split text per slide or page into structured chunks with provenance tags."""
        chunks = []
        for slide_num, text in sorted(slides_text_map.items()):
            cleaned = text.strip()
            if not cleaned:
                continue
            # If slide text is long, sub-chunk it while keeping slide tag
            if len(cleaned) > 1000:
                words = cleaned.split()
                sub_chunks = [" ".join(words[i:i+150]) for i in range(0, len(words), 120)]
                for sub_idx, sc in enumerate(sub_chunks):
                    chunks.append({
                        "slide_number": slide_num,
                        "text": f"[Slide {slide_num}] {sc}",
                        "sub_index": sub_idx,
                    })
            else:
                chunks.append({
                    "slide_number": slide_num,
                    "text": f"[Slide {slide_num}] {cleaned}",
                    "sub_index": 0,
                })
        return chunks

    async def execute(
        self,
        ctx: AIRequestContext,
        gateway: Any,
    ) -> Tuple[Dict[str, Any], TokenUsage, Dict[str, str]]:
        doc_id = ctx.input.get("doc_id", str(uuid.uuid4()))
        raw_text = ctx.input.get("text", "")
        slides_map = ctx.input.get("slides_map") # Optional Dict[int, str]
        course_code = ctx.course.course_id or ctx.input.get("course_code", "GENERAL")
        institution_id = ctx.tenant.institution_id
        source = ctx.input.get("source", "lecture_slides")

        if not raw_text and not slides_map:
            raise ValueError("No text or slides_map provided for ingestion")

        # 1. Chunk text (either structured slide map or continuous document text)
        if slides_map:
            structured_chunks = self.chunk_slides(slides_map)
            chunk_texts = [c["text"] for c in structured_chunks]
        else:
            chunk_texts = self.chunk_text(raw_text)
            structured_chunks = [{"slide_number": i + 1, "text": ct, "sub_index": 0} for i, ct in enumerate(chunk_texts)]

        # 2. Generate vector embeddings in batch via Cohere
        vectors = await gateway.cohere_adapter.embed_texts(chunk_texts, input_type="search_document")

        # 3. Store in PostgreSQL ai.lexi_chunks with provenance
        with get_db_session() as session:
            for i, (item, vec) in enumerate(zip(structured_chunks, vectors)):
                chunk_uuid = str(uuid.uuid5(uuid.NAMESPACE_URL, f"{doc_id}::chunk::{i}"))
                slide_num = item.get("slide_number", i + 1)
                
                existing = session.query(LexiChunk).filter(LexiChunk.id == chunk_uuid).first()
                if existing:
                    existing.chunk_text = item["text"]
                    existing.embedding = vec
                    existing.course = course_code
                    existing.institution_id = institution_id
                    existing.source = source
                else:
                    session.add(
                        LexiChunk(
                            id=chunk_uuid,
                            doc_id=doc_id,
                            institution_id=institution_id,
                            course=course_code,
                            chunk_index=i,
                            chunk_text=item["text"],
                            source=source,
                            embedding=vec,
                        )
                    )

        total_chars = sum(len(c) for c in chunk_texts)
        result = {
            "doc_id": doc_id,
            "course": course_code,
            "chunks_created": len(chunk_texts),
            "status": "indexed",
            "slide_count": len(slides_map) if slides_map else len(chunk_texts),
        }

        usage = TokenUsage(total_tokens=total_chars // 4)
        return result, usage, {"provider": "cohere", "model": "embed-multilingual-v3.0"}


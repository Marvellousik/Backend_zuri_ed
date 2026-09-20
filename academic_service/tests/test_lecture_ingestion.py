"""Unit tests for Lecture Ingestion Service."""
import pytest
from academic_service.services.lecture_ingestion_service import LectureIngestionService


def test_pdf_extraction_fallback():
    sample_text = "[Slide 1] Dynamic Programming Overview\n\n[Slide 2] Optimal Substructure and Memoization"
    slides = LectureIngestionService.extract_text_from_pdf_bytes(sample_text.encode("utf-8"))
    assert len(slides) >= 1
    assert any("Dynamic Programming" in text for text in slides.values())


def test_plain_text_chunking():
    raw_notes = "Topic: Binary Trees\n\nSubtopic: In-order traversal and balanced AVL rotations."
    slides = LectureIngestionService.extract_text_from_pdf_bytes(raw_notes.encode("utf-8"))
    assert len(slides) >= 1


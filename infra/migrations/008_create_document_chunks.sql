-- Migration: Create zuri_chunks and reading_document_chunks tables for pgvector similarity search
-- Shared across AI Service, Ingestion, Retrieval, and Reading Assistant pipelines.

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Create ai schema
CREATE SCHEMA IF NOT EXISTS ai;

-- 1. Document chunks table (Cohere embed-multilingual-v3.0 = 1024 dimensions)
CREATE TABLE IF NOT EXISTS ai.zuri_chunks (
    id VARCHAR PRIMARY KEY,
    doc_id VARCHAR NOT NULL,
    course VARCHAR NOT NULL,
    chunk_index INTEGER NOT NULL,
    chunk_text TEXT NOT NULL,
    source VARCHAR NOT NULL DEFAULT 'uploaded_note',
    embedding vector(1024) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- B-tree indexes for fast lookup by doc and course
CREATE INDEX IF NOT EXISTS idx_zuri_chunks_doc_id ON ai.zuri_chunks(doc_id);
CREATE INDEX IF NOT EXISTS idx_zuri_chunks_course ON ai.zuri_chunks(course);

-- HNSW cosine distance index for vector similarity search (1024 dimensions)
CREATE INDEX IF NOT EXISTS idx_zuri_chunks_embedding_hnsw 
    ON ai.zuri_chunks USING hnsw (embedding vector_cosine_ops) 
    WITH (m = 16, ef_construction = 64);

-- 2. Reading Assistant document chunks table (Gemini gemini-embedding-001 = 768 dimensions)
CREATE TABLE IF NOT EXISTS ai.reading_document_chunks (
    id VARCHAR PRIMARY KEY,
    doc_id VARCHAR NOT NULL,
    chunk_index INTEGER NOT NULL,
    chunk_text TEXT NOT NULL,
    embedding vector(768) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- B-tree index for doc_id
CREATE INDEX IF NOT EXISTS idx_reading_chunks_doc_id ON ai.reading_document_chunks(doc_id);

-- HNSW cosine distance index for vector similarity search (768 dimensions)
CREATE INDEX IF NOT EXISTS idx_reading_chunks_embedding_hnsw 
    ON ai.reading_document_chunks USING hnsw (embedding vector_cosine_ops) 
    WITH (m = 16, ef_construction = 64);

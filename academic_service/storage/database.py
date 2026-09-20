"""
Database connection, session management, and schema initialization for Academic Service.
"""
import os
import logging
from contextlib import contextmanager
from typing import Generator
from sqlalchemy import create_engine, text
from sqlalchemy.orm import sessionmaker, Session

from academic_service.models.orm import Base

logger = logging.getLogger(__name__)

DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql://lexiassist:lexiassist_secret@localhost:5432/lexiassist"
)

if DATABASE_URL.startswith("postgres://"):
    DATABASE_URL = DATABASE_URL.replace("postgres://", "postgresql://", 1)

try:
    engine = create_engine(
        DATABASE_URL,
        pool_size=10,
        max_overflow=20,
        pool_timeout=30,
        pool_recycle=1800,
        pool_pre_ping=True,
    )
except Exception as e:
    logger.warning(f"Using in-memory SQLite fallback due to database driver issue: {e}")
    engine = create_engine("sqlite:///:memory:")

SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)


def init_db():
    """Ensure academic schema and tables are initialized."""
    try:
        with engine.connect() as conn:
            conn.execute(text("CREATE SCHEMA IF NOT EXISTS academic"))
            conn.commit()
            
        Base.metadata.create_all(bind=engine)
        logger.info("✅ Database schema 'academic' and tables initialized successfully")
    except Exception as e:
        logger.error(f"❌ Academic database initialization failed: {e}")


@contextmanager
def get_db_session() -> Generator[Session, None, None]:
    """Context manager for scoped database transactions."""
    session: Session = SessionLocal()
    try:
        yield session
        session.commit()
    except Exception:
        session.rollback()
        raise
    finally:
        session.close()


def get_db():
    """FastAPI dependency for DB sessions."""
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()

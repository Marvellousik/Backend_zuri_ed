"""
Database connection, session lifecycle, and schema initialization for LexiAssist AI Service.
"""
import os
import logging
from contextlib import contextmanager
from typing import Generator
from sqlalchemy import create_engine, text
from sqlalchemy.orm import sessionmaker, Session

from ai_service.storage.models import Base

logger = logging.getLogger(__name__)

DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql://lexiassist:lexiassist_secret@localhost:5432/lexiassist"
)

# Fix postgresql:// vs postgres:// URL prefix for SQLAlchemy if needed
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
    from sqlalchemy.pool import StaticPool
    from sqlalchemy import event
    engine = create_engine(
        "sqlite://",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )
    @event.listens_for(engine, "connect")
    def do_connect(dbapi_connection, connection_record):
        cursor = dbapi_connection.cursor()
        cursor.execute("ATTACH DATABASE ':memory:' AS ai")
        cursor.close()

SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)


def init_db():
    """Ensure vector extension, ai schema, and tables are initialized."""
    try:
        if engine.dialect.name != "sqlite":
            with engine.connect() as conn:
                try:
                    conn.execute(text("CREATE EXTENSION IF NOT EXISTS vector"))
                except Exception as e:
                    logger.warning(f"Could not enable 'vector' extension: {e}")
                conn.execute(text("CREATE SCHEMA IF NOT EXISTS ai"))
                conn.commit()
        Base.metadata.create_all(bind=engine)
        logger.info("✅ Database schema 'ai' and tables initialized successfully")
    except Exception as e:
        logger.error(f"❌ Database initialization failed: {e}")
        # Fail open for development/offline if database is temporarily unavailable


@contextmanager
def get_db_session() -> Generator[Session, None, None]:
    """Context manager for scoped database sessions with automatic commit/rollback."""
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

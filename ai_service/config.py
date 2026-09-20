"""
Configuration settings for LexiAssist AI Microservice.
"""
import os
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    service_name: str = "ai-microservice"
    environment: str = os.getenv("ENVIRONMENT", "development")
    port: int = int(os.getenv("PORT", "5000"))
    log_level: str = os.getenv("LOG_LEVEL", "INFO")
    
    # Internal Authentication
    internal_api_key: str = os.getenv("INTERNAL_API_KEY", "dev-internal-key")
    
    # Databases
    database_url: str = os.getenv(
        "DATABASE_URL",
        "postgresql://lexiassist:lexiassist_secret@localhost:5432/lexiassist"
    )
    redis_url: str = os.getenv("REDIS_URL", "redis://localhost:6379")
    
    # Model Provider Keys
    gemini_api_key: str = os.getenv("GEMINI_API_KEY", "")
    cohere_api_key: str = os.getenv("COHERE_API_KEY", "")
    openrouter_api_key: str = os.getenv("OPENROUTER_API_KEY", "")
    
    # Defaults
    default_model: str = os.getenv("DEFAULT_MODEL", "openrouter/auto")

    class Config:
        env_file = ".env"
        extra = "ignore"


settings = Settings()

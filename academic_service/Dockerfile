FROM python:3.12-slim

WORKDIR /app

# Install system dependencies (libpq-dev for PostgreSQL, curl for healthchecks)
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    gcc \
    libpq-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements
COPY academic_service/requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

# Copy source code
COPY academic_service /app/academic_service
COPY ai_service /app/ai_service

# Create non-root user
RUN groupadd -g 1001 appgroup && \
    useradd -u 1001 -g appgroup -s /bin/sh -m appuser && \
    chown -R appuser:appgroup /app

USER appuser

ENV PYTHONPATH=/app
ENV PORT=8086

EXPOSE 8086

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD curl -f http://localhost:8086/health || exit 1

CMD ["python", "-m", "uvicorn", "academic_service.cmd.main:app", "--host", "0.0.0.0", "--port", "8086"]

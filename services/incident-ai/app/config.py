from __future__ import annotations

import os
from dataclasses import dataclass


@dataclass(slots=True)
class Settings:
    port: int = 8087
    database_url: str = ""
    nats_url: str = "nats://localhost:4222"
    observability_url: str = "http://localhost:8086"
    deployment_engine_url: str = "http://localhost:8085"
    qdrant_url: str = "http://localhost:6333"
    qdrant_collection: str = "helmix-incidents"
    llm_provider: str = "mock"
    audit_log_path: str = "/tmp/incident-ai-audit.log"


def load_settings() -> Settings:
    settings = Settings(
        port=int(os.getenv("PORT", "8087")),
        database_url=os.getenv("DATABASE_URL", "").strip(),
        nats_url=os.getenv("NATS_URL", "nats://localhost:4222").strip(),
        observability_url=os.getenv("OBSERVABILITY_SERVICE_URL", "http://localhost:8086").strip(),
        deployment_engine_url=os.getenv("DEPLOYMENT_ENGINE_SERVICE_URL", "http://localhost:8085").strip(),
        qdrant_url=os.getenv("QDRANT_URL", "http://localhost:6333").strip(),
        qdrant_collection=os.getenv("QDRANT_COLLECTION", "helmix-incidents").strip() or "helmix-incidents",
        llm_provider=os.getenv("HELMIX_LLM_PROVIDER", "mock").strip().lower() or "mock",
        audit_log_path=os.getenv("INCIDENT_AI_AUDIT_LOG_PATH", "/tmp/incident-ai-audit.log").strip(),
    )
    if not settings.database_url:
        raise ValueError("DATABASE_URL is required")
    return settings
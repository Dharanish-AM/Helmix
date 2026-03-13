from __future__ import annotations

import asyncio
import json
from contextlib import asynccontextmanager, suppress

import asyncpg
import structlog
from fastapi import FastAPI, HTTPException, Query
from nats.aio.client import Client as NATS

from .classifier import classify_stack
from .config import Settings, load_settings
from .context_clients import ContextClients
from .llm.provider import get_provider
from .models import AlertEvent, ClassificationRequest, ClassificationResponse, IncidentListResponse, IncidentSummary, ManualActionRequest, ManualActionResponse
from .repository import IncidentRepository
from .service import IncidentService


class NATSPublisher:
    def __init__(self, client: NATS) -> None:
        self._client = client

    async def publish(self, subject: str, payload: dict[str, object]) -> None:
        await self._client.publish(subject, json.dumps(payload).encode("utf-8"))


def create_app() -> FastAPI:
    settings = load_settings()
    logger = structlog.get_logger("incident-ai")

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        pool = await asyncpg.create_pool(settings.database_url, min_size=1, max_size=5)
        nats_client = NATS()
        await nats_client.connect(settings.nats_url)

        repository = IncidentRepository(pool)
        service = IncidentService(
            repository=repository,
            provider=get_provider(settings.llm_provider),
            context_clients=ContextClients(settings.observability_url, settings.deployment_engine_url),
            publisher=NATSPublisher(nats_client),
            deployment_engine_url=settings.deployment_engine_url,
            audit_log_path=settings.audit_log_path,
        )

        async def on_alert(message):
            try:
                payload = json.loads(message.data.decode("utf-8"))
                event = AlertEvent.model_validate(payload)
                await service.process_alert(event)
            except Exception as exc:  # pragma: no cover - runtime logging path
                logger.error("incident processing failed", error=str(exc))

        subscription = await nats_client.subscribe("alert.fired", cb=on_alert)

        app.state.settings = settings
        app.state.pool = pool
        app.state.nats = nats_client
        app.state.subscription = subscription
        app.state.service = service

        yield

        await subscription.unsubscribe()
        await nats_client.drain()
        await pool.close()

    app = FastAPI(title="Helmix Incident AI", lifespan=lifespan)

    @app.get("/health")
    async def health() -> dict[str, str]:
        return {"status": "ok", "service": "incident-ai", "version": "0.1.0"}

    @app.get("/projects/{project_id}", response_model=IncidentListResponse)
    async def list_project_incidents(
        project_id: str,
        limit: int = Query(20, ge=1, le=200),
        offset: int = Query(0, ge=0),
    ) -> IncidentListResponse:
        return await app.state.service.list_incidents(project_id, limit, offset)

    @app.get("/{incident_id}", response_model=IncidentSummary)
    async def incident_detail(incident_id: str) -> IncidentSummary:
        incident = await app.state.service.get_incident(incident_id)
        if incident is None:
            raise HTTPException(status_code=404, detail="incident not found")
        return incident

    @app.post("/{incident_id}/actions", response_model=ManualActionResponse)
    async def incident_action(incident_id: str, request: ManualActionRequest) -> ManualActionResponse:
        try:
            return await app.state.service.trigger_manual_action(incident_id, request)
        except LookupError as exc:
            raise HTTPException(status_code=404, detail=str(exc)) from exc

    @app.get("/{incident_id}/similar")
    async def similar_incidents(incident_id: str) -> list[dict[str, object]]:
        incident = await app.state.service.get_incident(incident_id)
        if incident is None:
            raise HTTPException(status_code=404, detail="incident not found")
        return await app.state.service.get_similar(incident_id)

    @app.post("/classify", response_model=ClassificationResponse)
    async def classify(request: ClassificationRequest) -> ClassificationResponse:
        return classify_stack(request)

    return app
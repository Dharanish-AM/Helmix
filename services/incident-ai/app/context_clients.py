from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

import httpx

from .models import DeploymentContext


class ContextClients:
    def __init__(self, observability_url: str, deployment_engine_url: str) -> None:
        self._observability_url = observability_url.rstrip("/")
        self._deployment_engine_url = deployment_engine_url.rstrip("/")

    async def fetch_metrics(self, client: httpx.AsyncClient, project_id: str) -> list[dict[str, Any]]:
        try:
            response = await client.get(f"{self._observability_url}/metrics/{project_id}")
            response.raise_for_status()
            payload = response.json()
            return payload if isinstance(payload, list) else []
        except Exception:
            return []

    async def fetch_recent_deployments(self, client: httpx.AsyncClient, project_id: str) -> list[DeploymentContext]:
        """Fetch the last 5 deployments for the project and return structured contexts."""
        try:
            response = await client.get(
                f"{self._deployment_engine_url}/deployments",
                params={"project_id": project_id, "limit": 5},
            )
            response.raise_for_status()
            payload = response.json()
            if not isinstance(payload, list):
                return []
            return [self._parse_deployment(raw) for raw in payload]
        except Exception:
            return []

    def _parse_deployment(self, raw: dict[str, Any]) -> DeploymentContext:
        """Normalise a raw deployment record from the deployment-engine into a DeploymentContext."""
        started_at_raw: str = raw.get("started_at") or raw.get("created_at", "")
        minutes_since: float | None = None
        if started_at_raw:
            try:
                started_dt = datetime.fromisoformat(started_at_raw.replace("Z", "+00:00"))
                delta = datetime.now(timezone.utc) - started_dt
                minutes_since = round(delta.total_seconds() / 60, 1)
            except ValueError:
                pass
        return DeploymentContext(
            deployment_id=str(raw.get("id") or raw.get("deployment_id", "")),
            status=str(raw.get("status", "")),
            started_at=started_at_raw,
            image=str(raw.get("image", "")),
            environment=str(raw.get("environment", "")),
            minutes_since_deploy=minutes_since,
        )

    async def fetch_logs(self, client: httpx.AsyncClient, project_id: str) -> list[str]:
        _ = client, project_id
        return []

    async def fetch_similar_incidents(self, client: httpx.AsyncClient, project_id: str) -> list[dict[str, Any]]:
        _ = client, project_id
        return []
from __future__ import annotations

import json
from typing import Any

import asyncpg

from .models import IncidentSummary


class IncidentRepository:
    def __init__(self, pool: asyncpg.Pool) -> None:
        self._pool = pool

    async def insert_incident(self, alert_id: str, project_id: str, diagnosis: dict[str, Any], actions: list[dict[str, Any]]) -> IncidentSummary:
        query = """
            INSERT INTO incidents (alert_id, project_id, ai_diagnosis, ai_actions)
            VALUES ($1, $2, $3::jsonb, $4::jsonb)
            RETURNING id, alert_id, project_id, ai_diagnosis, ai_actions, resolved_at, created_at
        """
        async with self._pool.acquire() as connection:
            row = await connection.fetchrow(query, alert_id, project_id, json.dumps(diagnosis), json.dumps(actions))
        return IncidentSummary.model_validate(_normalize_incident_row(row))

    async def list_incidents(self, project_id: str, limit: int, offset: int) -> tuple[list[IncidentSummary], int]:
        items_query = """
            SELECT id, alert_id, project_id, ai_diagnosis, ai_actions, resolved_at, created_at
            FROM incidents
            WHERE project_id = $1
            ORDER BY created_at DESC
            LIMIT $2 OFFSET $3
        """
        total_query = """
            SELECT COUNT(*)
            FROM incidents
            WHERE project_id = $1
        """
        async with self._pool.acquire() as connection:
            rows = await connection.fetch(items_query, project_id, limit, offset)
            total = await connection.fetchval(total_query, project_id)
        items = [IncidentSummary.model_validate(_normalize_incident_row(row)) for row in rows]
        return items, int(total or 0)

    async def get_incident(self, incident_id: str) -> IncidentSummary | None:
        query = """
            SELECT id, alert_id, project_id, ai_diagnosis, ai_actions, resolved_at, created_at
            FROM incidents
            WHERE id = $1
        """
        async with self._pool.acquire() as connection:
            row = await connection.fetchrow(query, incident_id)
        return IncidentSummary.model_validate(_normalize_incident_row(row)) if row else None

    async def append_action(self, incident_id: str, entry: dict[str, Any]) -> IncidentSummary | None:
        query = """
            UPDATE incidents
            SET ai_actions = COALESCE(ai_actions, '[]'::jsonb) || $2::jsonb
            WHERE id = $1
            RETURNING id, alert_id, project_id, ai_diagnosis, ai_actions, resolved_at, created_at
        """
        async with self._pool.acquire() as connection:
            row = await connection.fetchrow(query, incident_id, json.dumps([entry]))
        return IncidentSummary.model_validate(_normalize_incident_row(row)) if row else None


def _normalize_incident_row(row: asyncpg.Record) -> dict[str, Any]:
    payload = dict(row)
    for key in ("id", "alert_id", "project_id"):
        if key in payload and payload[key] is not None:
            payload[key] = str(payload[key])

    for key, default in (("ai_diagnosis", {}), ("ai_actions", [])):
        value = payload.get(key)
        if value is None:
            payload[key] = default
        elif isinstance(value, str):
            payload[key] = json.loads(value)

    return payload
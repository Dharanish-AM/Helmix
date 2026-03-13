from __future__ import annotations

import hashlib
import math
from typing import Any

from qdrant_client import AsyncQdrantClient
from qdrant_client.http import models as qmodels


class NoOpIncidentMemoryStore:
    async def ensure_collection(self) -> None:
        return None

    async def search_similar(
        self,
        project_id: str,
        query_text: str,
        limit: int = 3,
        exclude_incident_id: str | None = None,
    ) -> list[dict[str, Any]]:
        _ = project_id, query_text, limit, exclude_incident_id
        return []

    async def upsert_incident_memory(
        self,
        incident_id: str,
        project_id: str,
        alert_id: str,
        summary: str,
        actions_taken: list[dict[str, Any]],
    ) -> None:
        _ = incident_id, project_id, alert_id, summary, actions_taken
        return None


class QdrantIncidentMemoryStore:
    VECTOR_SIZE = 64

    def __init__(self, url: str, collection: str) -> None:
        self._client = AsyncQdrantClient(url=url)
        self._collection = collection

    async def ensure_collection(self) -> None:
        collections = await self._client.get_collections()
        existing = {item.name for item in collections.collections}
        if self._collection in existing:
            return
        await self._client.create_collection(
            collection_name=self._collection,
            vectors_config=qmodels.VectorParams(
                size=self.VECTOR_SIZE,
                distance=qmodels.Distance.COSINE,
            ),
        )

    async def search_similar(
        self,
        project_id: str,
        query_text: str,
        limit: int = 3,
        exclude_incident_id: str | None = None,
    ) -> list[dict[str, Any]]:
        filter_conditions: list[qmodels.FieldCondition] = [
            qmodels.FieldCondition(
                key="project_id",
                match=qmodels.MatchValue(value=project_id),
            )
        ]
        must_not: list[qmodels.FieldCondition] = []
        if exclude_incident_id:
            must_not.append(
                qmodels.FieldCondition(
                    key="incident_id",
                    match=qmodels.MatchValue(value=exclude_incident_id),
                )
            )
        hits = await self._client.search(
            collection_name=self._collection,
            query_vector=self._embed_text(query_text),
            query_filter=qmodels.Filter(must=filter_conditions, must_not=must_not),
            limit=limit,
            with_payload=True,
        )
        results: list[dict[str, Any]] = []
        for hit in hits:
            if hit.score <= 0.7:
                continue
            payload = hit.payload or {}
            results.append(
                {
                    "incident_id": str(payload.get("incident_id", "")),
                    "score": float(hit.score),
                    "summary": str(payload.get("summary", "")),
                }
            )
        return results

    async def upsert_incident_memory(
        self,
        incident_id: str,
        project_id: str,
        alert_id: str,
        summary: str,
        actions_taken: list[dict[str, Any]],
    ) -> None:
        action_names = [str(action.get("action", "")) for action in actions_taken]
        memory_text = f"{summary}. Alert: {alert_id}. Actions: {', '.join(a for a in action_names if a)}"
        await self._client.upsert(
            collection_name=self._collection,
            points=[
                qmodels.PointStruct(
                    id=incident_id,
                    vector=self._embed_text(memory_text),
                    payload={
                        "incident_id": incident_id,
                        "project_id": project_id,
                        "alert_id": alert_id,
                        "summary": summary,
                        "actions": action_names,
                    },
                )
            ],
        )

    def _embed_text(self, text: str) -> list[float]:
        buckets = [0.0] * self.VECTOR_SIZE
        tokens = [token.strip().lower() for token in text.split() if token.strip()]
        if not tokens:
            return buckets
        for token in tokens:
            digest = hashlib.sha256(token.encode("utf-8")).digest()
            index = int.from_bytes(digest[:2], "big") % self.VECTOR_SIZE
            sign = 1.0 if digest[2] % 2 == 0 else -1.0
            buckets[index] += sign
        magnitude = math.sqrt(sum(value * value for value in buckets))
        if magnitude == 0:
            return buckets
        return [value / magnitude for value in buckets]
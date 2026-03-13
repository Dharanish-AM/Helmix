CREATE TABLE metric_snapshots (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id      UUID REFERENCES projects(id),
  captured_at     TIMESTAMPTZ NOT NULL,
  cpu_pct         DOUBLE PRECISION NOT NULL,
  memory_pct      DOUBLE PRECISION NOT NULL,
  req_per_sec     DOUBLE PRECISION NOT NULL,
  error_rate_pct  DOUBLE PRECISION NOT NULL,
  p99_latency_ms  DOUBLE PRECISION NOT NULL,
  pod_count       INTEGER NOT NULL,
  ready_pod_count INTEGER NOT NULL,
  pod_restarts    INTEGER NOT NULL DEFAULT 0,
  source          TEXT NOT NULL DEFAULT 'synthetic',
  created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_metric_snapshots_project_id_captured_at
  ON metric_snapshots(project_id, captured_at DESC);
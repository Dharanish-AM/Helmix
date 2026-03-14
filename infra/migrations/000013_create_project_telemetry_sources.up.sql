CREATE TABLE IF NOT EXISTS project_telemetry_sources (
  project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
  source_type TEXT NOT NULL DEFAULT 'helmix-json',
  metrics_url TEXT NOT NULL,
  scrape_interval_seconds INTEGER NOT NULL DEFAULT 30,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  last_scraped_at TIMESTAMPTZ,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_project_telemetry_sources_updated_at
  ON project_telemetry_sources(updated_at DESC);
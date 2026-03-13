CREATE TABLE alerts (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  UUID REFERENCES projects(id),
  severity    TEXT NOT NULL,
  title       TEXT NOT NULL,
  description TEXT,
  status      TEXT DEFAULT 'open',
  created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE incidents (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  alert_id     UUID REFERENCES alerts(id),
  project_id   UUID REFERENCES projects(id),
  ai_diagnosis JSONB,
  ai_actions   JSONB,
  resolved_at  TIMESTAMPTZ,
  created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_alerts_project_id ON alerts(project_id);
CREATE INDEX idx_incidents_project_id ON incidents(project_id);

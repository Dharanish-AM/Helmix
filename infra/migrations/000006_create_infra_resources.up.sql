CREATE TABLE infra_resources (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID REFERENCES projects(id),
  type       TEXT NOT NULL,
  name       TEXT NOT NULL,
  namespace  TEXT NOT NULL,
  manifest   JSONB,
  status     TEXT DEFAULT 'pending',
  created_at TIMESTAMPTZ DEFAULT now()
);

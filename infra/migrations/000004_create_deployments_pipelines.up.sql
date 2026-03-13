CREATE TABLE deployments (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  repo_id     UUID REFERENCES repos(id),
  commit_sha  TEXT NOT NULL,
  branch      TEXT NOT NULL,
  status      TEXT NOT NULL,
  environment TEXT NOT NULL,
  image_tag   TEXT,
  deployed_at TIMESTAMPTZ,
  created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE pipelines (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  repo_id      UUID REFERENCES repos(id),
  run_id       TEXT NOT NULL,
  status       TEXT NOT NULL,
  stages       JSONB,
  triggered_by TEXT,
  created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_deployments_repo_id ON deployments(repo_id);

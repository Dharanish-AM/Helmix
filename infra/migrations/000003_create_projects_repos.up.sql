CREATE TABLE projects (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     UUID REFERENCES organizations(id),
  name       TEXT NOT NULL,
  slug       TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE (org_id, slug)
);

CREATE TABLE repos (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id     UUID REFERENCES projects(id),
  github_repo    TEXT NOT NULL,
  default_branch TEXT DEFAULT 'main',
  detected_stack JSONB,
  connected_at   TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_repos_project_id ON repos(project_id);

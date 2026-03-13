CREATE TABLE organizations (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL,
  slug       TEXT UNIQUE NOT NULL,
  owner_id   UUID REFERENCES users(id),
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE org_members (
  org_id  UUID REFERENCES organizations(id),
  user_id UUID REFERENCES users(id),
  role    TEXT NOT NULL,
  PRIMARY KEY (org_id, user_id)
);

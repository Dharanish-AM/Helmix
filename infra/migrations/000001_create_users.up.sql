CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  github_id   BIGINT UNIQUE NOT NULL,
  username    TEXT NOT NULL,
  email       TEXT NOT NULL,
  avatar_url  TEXT,
  created_at  TIMESTAMPTZ DEFAULT now()
);

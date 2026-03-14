CREATE TABLE IF NOT EXISTS audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  service TEXT NOT NULL,
  event_type TEXT NOT NULL,
  actor_user_id UUID REFERENCES users(id),
  actor_org_id UUID REFERENCES organizations(id),
  request_id TEXT,
  ip_address TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_service_event_created_at ON audit_logs(service, event_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_user_id_created_at ON audit_logs(actor_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_org_id_created_at ON audit_logs(actor_org_id, created_at DESC);
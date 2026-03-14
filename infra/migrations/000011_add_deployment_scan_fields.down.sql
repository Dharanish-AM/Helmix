ALTER TABLE deployments
  DROP COLUMN IF EXISTS accept_risk,
  DROP COLUMN IF EXISTS scan_results;

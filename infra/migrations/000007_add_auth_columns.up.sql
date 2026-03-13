ALTER TABLE users
  ADD COLUMN IF NOT EXISTS github_token_nonce BYTEA,
  ADD COLUMN IF NOT EXISTS github_token_ciphertext BYTEA,
  ADD COLUMN IF NOT EXISTS github_token_updated_at TIMESTAMPTZ;
ALTER TABLE users
  DROP COLUMN IF EXISTS github_token_updated_at,
  DROP COLUMN IF EXISTS github_token_ciphertext,
  DROP COLUMN IF EXISTS github_token_nonce;
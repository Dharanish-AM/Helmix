#!/usr/bin/env sh
set -eu

VAULT_ADDR="${VAULT_ADDR:-http://vault:8200}"
VAULT_TOKEN="${VAULT_TOKEN:-root}"
VAULT_KV_MOUNT="${VAULT_KV_MOUNT:-secret}"
VAULT_ROLE_NAME="${VAULT_ROLE_NAME:-helmix-auth}"
VAULT_POLICY_NAME="${VAULT_POLICY_NAME:-helmix-auth}"
VAULT_APPROLE_ROLE_ID="${VAULT_APPROLE_ROLE_ID:-helmix-auth-role-id}"
VAULT_APPROLE_SECRET_ID="${VAULT_APPROLE_SECRET_ID:-helmix-auth-secret-id}"

export VAULT_ADDR
export VAULT_TOKEN

echo "waiting for vault at ${VAULT_ADDR}..."
for _ in $(seq 1 60); do
  if vault status >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

if ! vault status >/dev/null 2>&1; then
  echo "vault did not become ready in time"
  exit 1
fi

vault auth enable approle >/dev/null 2>&1 || true

if ! vault secrets list -format=json | grep -q "\"${VAULT_KV_MOUNT}/\""; then
  vault secrets enable -path="${VAULT_KV_MOUNT}" kv-v2 >/dev/null
fi

cat > /tmp/helmix-auth-policy.hcl <<EOF
path "${VAULT_KV_MOUNT}/data/*" {
  capabilities = ["create", "read", "update", "delete"]
}

path "${VAULT_KV_MOUNT}/metadata/*" {
  capabilities = ["read", "list", "delete"]
}
EOF

vault policy write "${VAULT_POLICY_NAME}" /tmp/helmix-auth-policy.hcl >/dev/null
vault write auth/approle/role/"${VAULT_ROLE_NAME}" token_policies="${VAULT_POLICY_NAME}" token_ttl=1h token_max_ttl=4h >/dev/null
vault write auth/approle/role/"${VAULT_ROLE_NAME}"/role-id role_id="${VAULT_APPROLE_ROLE_ID}" >/dev/null
if ! custom_secret_out=$(vault write auth/approle/role/"${VAULT_ROLE_NAME}"/custom-secret-id secret_id="${VAULT_APPROLE_SECRET_ID}" 2>&1); then
  case "${custom_secret_out}" in
    *"SecretID is already registered"*)
      echo "vault approle secret-id already registered; reusing existing secret-id"
      ;;
    *)
      echo "${custom_secret_out}"
      exit 1
      ;;
  esac
fi

echo "vault approle bootstrap completed"

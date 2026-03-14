# helmix-cli

`helmix-cli` is the Helmix command line interface for gateway-backed operations.

## Configuration

Configuration is loaded from flags, env vars, and optional config file:

- Flags: `--api-base-url`, `--token`, `--org-id`, `--timeout`, `--output`
- Env vars: `HELMIX_API_BASE_URL`, `HELMIX_TOKEN`, `HELMIX_ORG_ID`, `HELMIX_TIMEOUT`, `HELMIX_OUTPUT`
- Config file: `helmix.yaml` in current directory or `~/.config/helmix/helmix.yaml`

## Common Usage

```bash
# health
helmix health

# auth
helmix auth me --token "$HELMIX_TOKEN"

# orgs
helmix orgs create --name "Acme" --token "$HELMIX_TOKEN"
helmix orgs members --token "$HELMIX_TOKEN"

# secrets
helmix secrets set --service deployment-engine --key registry_token --value abc --token "$HELMIX_TOKEN"
helmix secrets get --service deployment-engine --key registry_token --token "$HELMIX_TOKEN"

# repos
helmix repos connect --github-repo owner/repo --default-branch main --token "$HELMIX_TOKEN"
helmix repos list --limit 20 --token "$HELMIX_TOKEN"

# infra + pipelines
helmix infra generate --project-slug demo --runtime node --framework nextjs --provider docker --token "$HELMIX_TOKEN"
helmix pipelines generate --project-slug demo --runtime node --framework nextjs --provider github-actions --token "$HELMIX_TOKEN"

# deployments
helmix deployments start --repo-id REPO_ID --commit-sha abc123 --branch main --environment staging --token "$HELMIX_TOKEN"
helmix deployments list --project-id PROJECT_ID --token "$HELMIX_TOKEN"

# observability + incidents
helmix observability current --project-id PROJECT_ID --token "$HELMIX_TOKEN"
helmix incidents list --project-id PROJECT_ID --token "$HELMIX_TOKEN"
```

## Build

From repository root:

```bash
make cli-build
```

Artifacts are written to `dist/`.

# Helmix Terraform Modules

This directory contains Phase 4.3 Terraform scaffolding for multi-cloud infrastructure.

## Layout

- `modules/vpc`: Shared network interface with provider-specific identifiers.
- `modules/kubernetes`: Kubernetes cluster abstraction for EKS/GKE/AKS.
- `modules/database`: PostgreSQL abstraction for RDS/Cloud SQL/Azure DB.
- `modules/cache`: Redis abstraction for ElastiCache/Memorystore/Azure Cache.
- `modules/registry`: Container registry abstraction for ECR/Artifact Registry/ACR.
- `environments/dev`: Local-focused stack (k3d mode, no cloud resources).
- `environments/staging`: Single-AZ-like lower-cost baseline.
- `environments/production`: Multi-AZ higher-availability baseline.

## Usage

```bash
make tf-plan env=staging
make tf-apply env=staging APPROVED=true
make tf-destroy env=dev APPROVED=true
```

Notes:
- `tf-apply` and `tf-destroy` require `APPROVED=true`.
- `tf-destroy` is intentionally limited to `env=dev`.

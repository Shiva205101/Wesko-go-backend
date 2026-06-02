# Wesko Production CI/CD

This directory is the operational baseline for the Wesko backend:

- GitHub Actions is the source-of-truth pipeline.
- Artifact Registry stores immutable Docker images in `asia-south1-docker.pkg.dev/eighth-brace-379306/wesko-backend`.
- `develop` deploys to the staging Cloud Run service.
- `main` deploys to the production Cloud Run service.
- version tags like `v1.4.0` publish semantic image tags in addition to the commit SHA tag.

## Files

- `.github/workflows/deploy.yml`: quality gates, dependency review, image build, signing, vulnerability scan, and Cloud Run deployment.
- `.github/dependabot.yml`: weekly updates for Go modules and GitHub Actions.
- `Dockerfile`: production multi-stage build for Go 1.25 with a distroless runtime image.
- `deployment/cloudrun/env/*.env`: non-secret runtime config split by environment.
- `deployment/cloudrun/secrets/*.env`: Secret Manager bindings per environment.
- `deployment/cloudbuild/manual-release.yaml`: Cloud Build fallback for manual emergency releases.
- `scripts/bootstrap-gcp.sh`: fresh-project bootstrap for APIs, Artifact Registry, service accounts, Workload Identity Federation, and baseline secret IAM.
- `scripts/sync-secret.sh`: rotates Secret Manager values from GitHub environment secrets during deployment.
- `scripts/deploy-cloudrun.sh`: authoritative Cloud Run deployment script used by GitHub Actions and operators.
- `scripts/rollback-cloudrun.sh`: traffic rollback to a prior Cloud Run revision.
- `scripts/check-vulnerabilities.sh`: on-demand vulnerability scan for the built image digest.

## Step-by-Step Setup

1. Create or select the GCP project and set it as default:
   `gcloud config set project eighth-brace-379306`

2. Export the GitHub repository coordinates and bootstrap the platform:
   `GITHUB_OWNER=change-me GITHUB_REPO=change-me ./scripts/bootstrap-gcp.sh`

3. In GitHub, create two environments:
   `staging`
   `production`

4. Add these GitHub environment secrets to both `staging` and `production`:
   `GCP_WORKLOAD_IDENTITY_PROVIDER`
   `GCP_SERVICE_ACCOUNT`
   `DATABASE_URL`
   `AUTH_JWE_KEY`

   `build-and-publish` now also uses GitHub Environments:
   `develop` uses `staging`
   `main` and `v*` tags use `production`

   That means the two GCP auth secrets must exist in both environments, not only in one of them.

   `DATABASE_URL` should be a PostgreSQL connection URL such as `postgres://user:password@host:5432/dbname?sslmode=require`.

   `AUTH_JWE_KEY` must be exactly 32 bytes because the current application uses it as the JWE key.

   The deployed app now reads only `AUTH_JWE_KEY`. The underlying Secret Manager resource name remains `wesko-jwt-secret-<env>` so you do not need to recreate secrets in GCP.

5. Set `GCP_WORKLOAD_IDENTITY_PROVIDER` to the provider name printed by `scripts/bootstrap-gcp.sh`.

6. Set `GCP_SERVICE_ACCOUNT` to:
   `wesko-github-deployer@eighth-brace-379306.iam.gserviceaccount.com`

7. Edit the environment files under `deployment/cloudrun/env/` and replace every `change-me-*` placeholder with real non-secret values.

   Set `CLOUD_SQL_CONNECTION_NAME` to the Cloud SQL instance connection name in the format `PROJECT_ID:REGION:INSTANCE_ID`.

   Set `VPC_CONNECTOR` to your Serverless VPC Access connector name if the service needs Redis or other private VPC resources.

8. If you need optional runtime secrets beyond the core list, create them in Secret Manager and then uncomment or add them in `deployment/cloudrun/secrets/staging.env` and `deployment/cloudrun/secrets/production.env`.

   Razorpay is optional right now and should stay commented out until that integration is actually implemented.

   If you enable Redis AUTH, map the secret to `REDIS_PASS`.

   The current codebase also expects working Redis connectivity and may need Google OAuth or Twilio values if you enable those flows.

9. Protect `main` and `develop` in GitHub:
   require pull requests
   require the `quality` job to pass
   require the `dependency-review` job to pass
   block force pushes
   require linear history

10. Push a branch and open a pull request. The PR runs formatting, vet, tests, and dependency review.

11. Merge into `develop` to deploy staging automatically.

12. Merge into `main` to deploy production automatically.

13. Create a version tag like `v1.0.0` on the production commit when you want a semantic image tag published.

## IAM Baseline

Grant the GitHub deployer service account:

- `roles/run.admin`
- `roles/artifactregistry.writer` on the `wesko-backend` repository
- `roles/ondemandscanning.admin`
- `roles/iam.serviceAccountUser` on the runtime service account
- `roles/secretmanager.secretVersionAdder` on each runtime secret

Grant the runtime service account:

- `roles/artifactregistry.reader` on the `wesko-backend` repository
- `roles/secretmanager.secretAccessor` on each runtime secret

Grant the Workload Identity principal:

- `roles/iam.workloadIdentityUser` on the GitHub deployer service account

## Environment Strategy

- `local`: values live in `configs/.env.config`; start from `deployment/cloudrun/env/local.env.example`.
- `development`: manual or ephemeral environment; no automatic deployment is configured.
- `staging`: GitHub environment secrets plus `deployment/cloudrun/env/staging.env`.
- `production`: GitHub environment secrets plus `deployment/cloudrun/env/production.env`.

Non-secrets are versioned in git. Secrets are rotated into Secret Manager during deploys and consumed by Cloud Run from Secret Manager, not from plain Cloud Run environment values.

For Cloud SQL on Cloud Run, set `DATABASE_URL` to a socket-aware PostgreSQL URI such as:
`postgres://wesko_app:DB_PASSWORD@/wesko?host=/cloudsql/eighth-brace-379306:asia-south1:wesko-postgres-prod&sslmode=disable`

`scripts/deploy-cloudrun.sh` reads `CLOUD_SQL_CONNECTION_NAME` and adds the Cloud SQL attachment during `gcloud run deploy`.

For Redis on Memorystore, `scripts/deploy-cloudrun.sh` also reads `VPC_CONNECTOR` and `VPC_EGRESS` and applies them to Cloud Run.

## Release Strategy

- Every push to `main` or `develop` must pass the same quality gate.
- Docker images are tagged with the full git SHA on every pushed branch build.
- semantic tags are published from Git tags that match `v*`.
- Cloud Run deployments use image digests, not mutable tags.

Because the Artifact Registry repository uses immutable tags, the pipeline does not publish a moving `latest` tag. If you truly need `latest`, you must either disable immutable tags for that repository or publish `latest` into a separate mutable repository.

## Rollback Strategy

List revisions:
`gcloud run revisions list --service=wesko-api --region=asia-south1`

Rollback to a known-good revision:
`./scripts/rollback-cloudrun.sh wesko-api REVISION_NAME`

Because Cloud Run revisions are immutable, rollback is a traffic switch rather than a rebuild.

## Security Controls

- Workload Identity Federation is used instead of long-lived service account JSON keys.
- Artifact Registry immutable tags should remain enabled.
- images are signed with Cosign in GitHub Actions.
- Binary Authorization can be enabled later by setting `BINARY_AUTH_POLICY` for deployments.
- dependency review runs on pull requests.
- Dependabot keeps modules and actions moving.
- secrets are versioned in Secret Manager so rotation and rollback stay explicit.

## Monitoring and Observability

- Application logs use `slog`; staging and production emit JSON for Cloud Logging.
- Cloud Run automatically sends request, container, and system logs to Cloud Logging.
- Error logs in stdout and stderr are surfaced in Error Reporting.
- `/health` is the liveness and startup probe target.
- `/ready` checks PostgreSQL and Redis reachability inside the running service.
- trace propagation is enabled so W3C `traceparent` headers can flow through the service.
- create Cloud Monitoring uptime checks against the public `/health` endpoint for staging and production.

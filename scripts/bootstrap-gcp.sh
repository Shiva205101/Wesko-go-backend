#!/usr/bin/env bash

set -euo pipefail

project_id="${PROJECT_ID:-eighth-brace-379306}"
region="${REGION:-asia-south1}"
repository="${REPOSITORY:-wesko-backend}"
github_owner="${GITHUB_OWNER:?GITHUB_OWNER is required}"
github_repo="${GITHUB_REPO:?GITHUB_REPO is required}"
workload_identity_pool_id="${WORKLOAD_IDENTITY_POOL_ID:-github}"
workload_identity_provider_id="${WORKLOAD_IDENTITY_PROVIDER_ID:-wesko}"
github_deployer_sa_name="${GITHUB_DEPLOYER_SA_NAME:-wesko-github-deployer}"
runtime_sa_name="${RUNTIME_SA_NAME:-wesko-runtime}"

project_number="$(gcloud projects describe "${project_id}" --format='value(projectNumber)')"
github_deployer_sa="${github_deployer_sa_name}@${project_id}.iam.gserviceaccount.com"
runtime_sa="${runtime_sa_name}@${project_id}.iam.gserviceaccount.com"
repo_resource="projects/${project_id}/locations/${region}/repositories/${repository}"
principal_set="principalSet://iam.googleapis.com/projects/${project_number}/locations/global/workloadIdentityPools/${workload_identity_pool_id}/attribute.repository/${github_owner}/${github_repo}"

gcloud services enable \
  artifactregistry.googleapis.com \
  binaryauthorization.googleapis.com \
  containeranalysis.googleapis.com \
  iam.googleapis.com \
  iamcredentials.googleapis.com \
  monitoring.googleapis.com \
  ondemandscanning.googleapis.com \
  run.googleapis.com \
  sqladmin.googleapis.com \
  secretmanager.googleapis.com \
  serviceusage.googleapis.com

if ! gcloud artifacts repositories describe "${repository}" --location="${region}" >/dev/null 2>&1; then
  gcloud artifacts repositories create "${repository}" \
    --repository-format=docker \
    --location="${region}" \
    --description="Wesko backend container images"
fi

gcloud artifacts repositories update "${repository}" \
  --location="${region}" \
  --immutable-tags

if ! gcloud iam service-accounts describe "${github_deployer_sa}" >/dev/null 2>&1; then
  gcloud iam service-accounts create "${github_deployer_sa_name}" \
    --display-name="Wesko GitHub deployer"
fi

if ! gcloud iam service-accounts describe "${runtime_sa}" >/dev/null 2>&1; then
  gcloud iam service-accounts create "${runtime_sa_name}" \
    --display-name="Wesko Cloud Run runtime"
fi

if ! gcloud iam workload-identity-pools describe "${workload_identity_pool_id}" --location=global >/dev/null 2>&1; then
  gcloud iam workload-identity-pools create "${workload_identity_pool_id}" \
    --location=global \
    --display-name="GitHub Actions Pool"
fi

if ! gcloud iam workload-identity-pools providers describe "${workload_identity_provider_id}" \
  --location=global \
  --workload-identity-pool="${workload_identity_pool_id}" >/dev/null 2>&1; then
  gcloud iam workload-identity-pools providers create-oidc "${workload_identity_provider_id}" \
    --location=global \
    --workload-identity-pool="${workload_identity_pool_id}" \
    --display-name="Wesko GitHub Provider" \
    --issuer-uri="https://token.actions.githubusercontent.com" \
    --attribute-mapping="google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository,attribute.ref=assertion.ref" \
    --attribute-condition="assertion.repository=='${github_owner}/${github_repo}'"
fi

gcloud iam service-accounts add-iam-policy-binding "${github_deployer_sa}" \
  --role="roles/iam.workloadIdentityUser" \
  --member="${principal_set}"

gcloud projects add-iam-policy-binding "${project_id}" \
  --member="serviceAccount:${github_deployer_sa}" \
  --role="roles/run.admin"

gcloud artifacts repositories add-iam-policy-binding "${repository}" \
  --location="${region}" \
  --member="serviceAccount:${github_deployer_sa}" \
  --role="roles/artifactregistry.writer"

gcloud projects add-iam-policy-binding "${project_id}" \
  --member="serviceAccount:${github_deployer_sa}" \
  --role="roles/ondemandscanning.admin"

gcloud iam service-accounts add-iam-policy-binding "${runtime_sa}" \
  --member="serviceAccount:${github_deployer_sa}" \
  --role="roles/iam.serviceAccountUser"

gcloud artifacts repositories add-iam-policy-binding "${repository}" \
  --location="${region}" \
  --member="serviceAccount:${runtime_sa}" \
  --role="roles/artifactregistry.reader"

gcloud projects add-iam-policy-binding "${project_id}" \
  --member="serviceAccount:${runtime_sa}" \
  --role="roles/cloudsql.client"

for secret_name in \
  wesko-database-url-staging \
  wesko-database-url-production \
  wesko-jwt-secret-staging \
  wesko-jwt-secret-production \
  wesko-razorpay-key-id-staging \
  wesko-razorpay-key-id-production \
  wesko-razorpay-key-secret-staging \
  wesko-razorpay-key-secret-production; do
  if ! gcloud secrets describe "${secret_name}" >/dev/null 2>&1; then
    gcloud secrets create "${secret_name}" --replication-policy=automatic
  fi

  gcloud secrets add-iam-policy-binding "${secret_name}" \
    --member="serviceAccount:${github_deployer_sa}" \
    --role="roles/secretmanager.secretVersionAdder"

  gcloud secrets add-iam-policy-binding "${secret_name}" \
    --member="serviceAccount:${runtime_sa}" \
    --role="roles/secretmanager.secretAccessor"
done

echo "GitHub deployer service account: ${github_deployer_sa}"
echo "Cloud Run runtime service account: ${runtime_sa}"
echo "Workload identity provider:"
echo "projects/${project_number}/locations/global/workloadIdentityPools/${workload_identity_pool_id}/providers/${workload_identity_provider_id}"
echo "Artifact Registry repository: ${repo_resource}"

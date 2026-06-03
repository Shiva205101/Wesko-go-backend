#!/usr/bin/env bash

set -euo pipefail

environment="${1:?environment is required}"
env_var_name="${2:?environment variable name is required}"
secret_name="${3:?secret name is required}"
project_id="${GCP_PROJECT_ID:-eighth-brace-379306}"

if [[ -z "${!env_var_name:-}" ]]; then
  echo "Skipping ${secret_name}; ${env_var_name} is empty for ${environment}."
  exit 0
fi

if ! gcloud secrets describe "${secret_name}" --project="${project_id}" >/dev/null 2>&1; then
  echo "Secret ${secret_name} does not exist in project ${project_id}. Run scripts/bootstrap-gcp.sh first (or check GCP_PROJECT_ID)." >&2
  exit 1
fi

printf '%s' "${!env_var_name}" | gcloud secrets versions add "${secret_name}" --project="${project_id}" --data-file=-
echo "Added a new version for ${secret_name} (${environment})."

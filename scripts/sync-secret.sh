#!/usr/bin/env bash

set -euo pipefail

environment="${1:?environment is required}"
env_var_name="${2:?environment variable name is required}"
secret_name="${3:?secret name is required}"

if [[ -z "${!env_var_name:-}" ]]; then
  echo "Skipping ${secret_name}; ${env_var_name} is empty for ${environment}."
  exit 0
fi

if ! gcloud secrets describe "${secret_name}" >/dev/null 2>&1; then
  echo "Secret ${secret_name} does not exist. Run scripts/bootstrap-gcp.sh first." >&2
  exit 1
fi

printf '%s' "${!env_var_name}" | gcloud secrets versions add "${secret_name}" --data-file=-
echo "Added a new version for ${secret_name} (${environment})."

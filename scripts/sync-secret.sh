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

tmp_err="$(mktemp)"
trap 'rm -f "${tmp_err}"' EXIT

if printf '%s' "${!env_var_name}" | gcloud secrets versions add "${secret_name}" --project="${project_id}" --data-file=- 2>"${tmp_err}"; then
  echo "Added a new version for ${secret_name} (${environment})."
  exit 0
fi

err="$(cat "${tmp_err}")"
if echo "${err}" | rg -q "NOT_FOUND|not found"; then
  echo "Secret ${secret_name} was not found in project ${project_id}. Create it (or run scripts/bootstrap-gcp.sh) and retry." >&2
  exit 1
fi
if echo "${err}" | rg -q "PERMISSION_DENIED|Permission denied|permissionDenied"; then
  echo "Permission denied adding a secret version for ${secret_name} in project ${project_id}." >&2
  echo "Grant the GitHub deployer service account roles/secretmanager.secretVersionAdder on this secret and retry." >&2
  exit 1
fi

echo "Failed to add secret version for ${secret_name} in project ${project_id}:" >&2
echo "${err}" >&2
exit 1

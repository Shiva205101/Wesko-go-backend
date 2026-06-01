#!/usr/bin/env bash

set -euo pipefail

service_name="${1:?service name is required}"
revision_name="${2:?revision name is required}"
project_id="${GCP_PROJECT_ID:-eighth-brace-379306}"
region="${GCP_REGION:-asia-south1}"

gcloud run services update-traffic "${service_name}" \
  --project="${project_id}" \
  --region="${region}" \
  --to-revisions="${revision_name}=100"

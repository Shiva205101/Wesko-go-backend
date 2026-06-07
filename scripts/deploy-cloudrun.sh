#!/usr/bin/env bash

set -euo pipefail

environment="${1:?environment is required}"
image_ref="${2:?image reference is required}"

project_id="${GCP_PROJECT_ID:-eighth-brace-379306}"
region="${GCP_REGION:-asia-south1}"
runtime_service_account="${RUNTIME_SERVICE_ACCOUNT:-wesko-runtime@${project_id}.iam.gserviceaccount.com}"
service_name="${SERVICE_NAME:-}"
git_commit_sha="${GIT_COMMIT_SHA:-unknown}"

if [[ -z "${service_name}" ]]; then
  case "${environment}" in
    production) service_name="wesko-api" ;;
    staging) service_name="wesko-api-staging" ;;
    development) service_name="wesko-api-development" ;;
    *)
      echo "Unknown environment ${environment}. Set SERVICE_NAME explicitly." >&2
      exit 1
      ;;
  esac
fi

case "${environment}" in
  production)
    min_instances="${MIN_INSTANCES:-1}"
    max_instances="${MAX_INSTANCES:-10}"
    ;;
  staging)
    min_instances="${MIN_INSTANCES:-0}"
    max_instances="${MAX_INSTANCES:-3}"
    ;;
  *)
    min_instances="${MIN_INSTANCES:-0}"
    max_instances="${MAX_INSTANCES:-1}"
    ;;
esac

env_dir="deployment/cloudrun/env"
secret_dir="deployment/cloudrun/secrets"
common_env_file="${env_dir}/common.env"
environment_env_file="${env_dir}/${environment}.env"
common_secret_file="${secret_dir}/common.env"
environment_secret_file="${secret_dir}/${environment}.env"

if [[ ! -f "${environment_env_file}" ]]; then
  echo "Missing environment file ${environment_env_file}" >&2
  exit 1
fi

tmp_env_file="$(mktemp)"
trap 'rm -f "${tmp_env_file}"' EXIT

trim() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "${value}"
}

yaml_escape() {
  printf '%s' "$1" | sed "s/'/''/g"
}

append_env_file() {
  local file_path="$1"
  [[ -f "${file_path}" ]] || return 0

  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="$(trim "${line}")"
    [[ -z "${line}" || "${line}" == \#* ]] && continue

    local key="${line%%=*}"
    local value="${line#*=}"
    key="$(trim "${key}")"
    value="$(trim "${value}")"
    value="${value//__ENV__/${environment}}"

    printf "%s: '%s'\n" "${key}" "$(yaml_escape "${value}")" >> "${tmp_env_file}"
  done < "${file_path}"
}

append_secret_file() {
  local file_path="$1"
  [[ -f "${file_path}" ]] || return 0

  local buffer="$2"
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="$(trim "${line}")"
    [[ -z "${line}" || "${line}" == \#* ]] && continue

    line="${line//__ENV__/${environment}}"
    if [[ -n "${buffer}" ]]; then
      buffer+=","
    fi
    buffer+="${line}"
  done < "${file_path}"

  printf '%s' "${buffer}"
}

append_env_file "${common_env_file}"
append_env_file "${environment_env_file}"

cloud_sql_connection_name="$(awk -F": " '$1=="CLOUD_SQL_CONNECTION_NAME" {gsub(/^'\''|'\''$/, "", $2); print $2}' "${tmp_env_file}" | tail -n1)"
redis_enabled="$(awk -F": " '$1=="REDIS_ENABLED" {gsub(/^'\''|'\''$/, "", $2); print tolower($2)}' "${tmp_env_file}" | tail -n1)"
vpc_connector="$(awk -F": " '$1=="VPC_CONNECTOR" {gsub(/^'\''|'\''$/, "", $2); print $2}' "${tmp_env_file}" | tail -n1)"
vpc_egress="$(awk -F": " '$1=="VPC_EGRESS" {gsub(/^'\''|'\''$/, "", $2); print $2}' "${tmp_env_file}" | tail -n1)"
vpc_network="$(awk -F": " '$1=="VPC_NETWORK" {gsub(/^'\''|'\''$/, "", $2); print $2}' "${tmp_env_file}" | tail -n1)"
vpc_subnet="$(awk -F": " '$1=="VPC_SUBNET" {gsub(/^'\''|'\''$/, "", $2); print $2}' "${tmp_env_file}" | tail -n1)"

if [[ "${redis_enabled}" != "true" ]]; then
  min_instances="${MIN_INSTANCES:-0}"
  max_instances="${MAX_INSTANCES:-1}"
fi

normalize_vpc_egress() {
  local raw="${1:-}"
  raw="$(trim "${raw}")"
  if [[ -z "${raw}" ]]; then
    echo ""
    return 0
  fi

  case "${raw}" in
    private-ranges-only|all|all-traffic)
      echo "${raw}"
      return 0
      ;;
  esac

  if [[ "${raw}" == */* ]]; then
    echo "Invalid VPC_EGRESS value: ${raw}" >&2
    echo "VPC_EGRESS must be one of: private-ranges-only, all-traffic (or all)." >&2
    echo "Do not set VPC_EGRESS to a CIDR. If you meant a subnet range, set VPC_SUBNET to the subnet NAME (e.g. default)." >&2
    exit 2
  fi

  echo "Invalid VPC_EGRESS value: ${raw}. Use private-ranges-only or all-traffic." >&2
  exit 2
}

vpc_egress="$(normalize_vpc_egress "${vpc_egress}")"

secrets_arg=""
secrets_arg="$(append_secret_file "${common_secret_file}" "${secrets_arg}")"
secrets_arg="$(append_secret_file "${environment_secret_file}" "${secrets_arg}")"

short_sha="$(printf '%s' "${git_commit_sha}" | cut -c1-12)"
labels="app=wesko,component=api,environment=${environment},managed-by=github-actions,commit-sha=${short_sha}"

cmd=(
  gcloud run deploy "${service_name}"
  --project="${project_id}"
  --region="${region}"
  --platform=managed
  --allow-unauthenticated
  --image="${image_ref}"
  --service-account="${runtime_service_account}"
  --execution-environment=gen2
  --ingress=all
  --cpu=1
  --memory=512Mi
  --concurrency=80
  --timeout=300
  --min-instances="${min_instances}"
  --max-instances="${max_instances}"
  --port=8080
  --cpu-boost
  --startup-probe=httpGet.path=/health,httpGet.port=8080,timeoutSeconds=5,periodSeconds=10,failureThreshold=3
  --liveness-probe=httpGet.path=/health,httpGet.port=8080,initialDelaySeconds=15,timeoutSeconds=5,periodSeconds=30,failureThreshold=3
  --env-vars-file="${tmp_env_file}"
  --update-labels="${labels}"
)

if [[ -n "${BINARY_AUTH_POLICY:-}" ]]; then
  cmd+=(--binary-authorization="${BINARY_AUTH_POLICY}")
fi

if [[ -n "${secrets_arg}" ]]; then
  cmd+=(--set-secrets="${secrets_arg}")
fi

if [[ -n "${cloud_sql_connection_name}" && "${cloud_sql_connection_name}" != change-me-* ]]; then
  cmd+=(--add-cloudsql-instances="${cloud_sql_connection_name}")
fi

if [[ -n "${vpc_connector}" && "${vpc_connector}" != change-me-* ]]; then
  cmd+=(--clear-network)
  cmd+=(--vpc-connector="${vpc_connector}")
  if [[ -n "${vpc_egress}" ]]; then
    cmd+=(--vpc-egress="${vpc_egress}")
  fi
elif [[ ( -n "${vpc_network}" && "${vpc_network}" != change-me-* ) || ( -n "${vpc_subnet}" && "${vpc_subnet}" != change-me-* ) ]]; then
  cmd+=(--clear-vpc-connector)
  if [[ -n "${vpc_network}" && "${vpc_network}" != change-me-* ]]; then
    cmd+=(--network="${vpc_network}")
  fi
  if [[ -n "${vpc_subnet}" && "${vpc_subnet}" != change-me-* ]]; then
    cmd+=(--subnet="${vpc_subnet}")
  fi
  cmd+=(--vpc-egress="${vpc_egress:-private-ranges-only}")
fi

"${cmd[@]}"

#!/usr/bin/env bash

set -euo pipefail

image_ref="${1:?image reference is required}"
scan_location="${SCAN_LOCATION:-asia}"
fail_on_severity="${FAIL_ON_SEVERITY:-CRITICAL}"
poll_attempts="${POLL_ATTEMPTS:-12}"
poll_interval_seconds="${POLL_INTERVAL_SECONDS:-10}"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required for vulnerability parsing." >&2
  exit 1
fi

scan_name="$(
  gcloud artifacts docker images scan "${image_ref}" \
    --remote \
    --location="${scan_location}" \
    --format='value(response.scan)'
)"

if [[ -z "${scan_name}" ]]; then
  echo "Failed to create an on-demand vulnerability scan for ${image_ref}." >&2
  exit 1
fi

report_file="$(mktemp)"
trap 'rm -f "${report_file}"' EXIT

for _ in $(seq 1 "${poll_attempts}"); do
  if gcloud artifacts docker images list-vulnerabilities "${scan_name}" \
    --location="${scan_location}" \
    --format=json >"${report_file}" 2>/dev/null; then
    break
  fi
  sleep "${poll_interval_seconds}"
done

critical_count="$(jq '[.[] | select((.effectiveSeverity // .severity // "") == "CRITICAL")] | length' "${report_file}")"
high_count="$(jq '[.[] | select((.effectiveSeverity // .severity // "") == "HIGH")] | length' "${report_file}")"

echo "Critical vulnerabilities: ${critical_count}"
echo "High vulnerabilities: ${high_count}"

case "${fail_on_severity}" in
  CRITICAL)
    [[ "${critical_count}" -eq 0 ]]
    ;;
  HIGH)
    [[ "${critical_count}" -eq 0 && "${high_count}" -eq 0 ]]
    ;;
  *)
    echo "Unsupported FAIL_ON_SEVERITY value: ${fail_on_severity}" >&2
    exit 1
    ;;
esac

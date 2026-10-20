#!/bin/sh
set -eu
component=${1:?usage: scan.sh COMPONENT}
case "$component" in api|worker|audio-concat|maintenance|web) ;; *) exit 2 ;; esac
IMAGE_SECURITY_DIR="$(pwd)/.image-security/$component"
IMAGE_SECURITY_CACHE_DIR="$(pwd)/.image-security/cache"
IMAGE_SECURITY_UID=$(id -u)
IMAGE_SECURITY_GID=$(id -g)
IMAGE_COMPONENT=$component
export IMAGE_SECURITY_DIR IMAGE_SECURITY_CACHE_DIR IMAGE_SECURITY_UID IMAGE_SECURITY_GID IMAGE_COMPONENT
mkdir -p "$IMAGE_SECURITY_CACHE_DIR"
test -f "$IMAGE_SECURITY_DIR/metadata.json"
test -f "$IMAGE_SECURITY_DIR/image.tar"
compose() { docker compose --env-file /dev/null -f infra/image-security/compose.yml "$@"; }
rm -f "$IMAGE_SECURITY_DIR/sbom.cdx.json" "$IMAGE_SECURITY_DIR/vulnerabilities.json" \
  "$IMAGE_SECURITY_DIR/summary.md" "$IMAGE_SECURITY_DIR/scan-status.json"
# Generate the inventory first. A vulnerability exit must not discard the SBOM.
sbom_status=0
compose run --rm -T --no-deps trivy image --input /input/image.tar \
  --pkg-types os,library --scanners vuln --severity UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL \
  --ignore-unfixed=false --ignorefile /dev/null --no-progress --format cyclonedx \
  --output /reports/sbom.cdx.json --exit-code 0 --timeout 20m || sbom_status=$?
vuln_status=0
compose run --rm -T --no-deps trivy image --input /input/image.tar \
  --pkg-types os,library --scanners vuln --severity UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL \
  --ignore-unfixed=false --ignorefile /dev/null --no-progress --exit-code 1 \
  --format json --list-all-pkgs --output /reports/vulnerabilities.json \
  --timeout 20m || vuln_status=$?
printf '{"sbom_exit_code":%s,"vulnerability_exit_code":%s}\n' \
  "$sbom_status" "$vuln_status" > "$IMAGE_SECURITY_DIR/scan-status.json"
report_status=0
compose run --rm -T --no-deps verifier python -B /tool/report.py verify || report_status=$?
if [ "$sbom_status" -ne 0 ] || [ "$report_status" -ne 0 ]; then exit 1; fi
# Exit 1 denotes findings only when the verifier has checked the complete
# reports, image identity and exact raw exit status. Other exits are errors.
case "$vuln_status" in 0|1) exit 0 ;; *) exit 1 ;; esac

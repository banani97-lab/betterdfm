#!/usr/bin/env bash
# Dependency vulnerability scan (NIST 800-171 3.11.2 / 3.14.5).
# Runs the language-native scanners across the monorepo. ECR image scanning
# (scan-on-push) covers container OS/base-layer CVEs separately; this covers
# application dependencies. Run before each release; record findings + actions.
#
# Exit non-zero if any scanner reports findings, so it can gate a release.
set -uo pipefail
cd "$(dirname "$0")/.."
rc=0

echo "== Go: govulncheck (engine, api, worker) =="
if command -v govulncheck >/dev/null 2>&1; then
  for d in engine/dfm-engine apps/api workers/dfm-worker; do
    echo "-- $d --"; ( cd "$d" && govulncheck ./... ) || rc=1
  done
else
  echo "govulncheck not installed: go install golang.org/x/vuln/cmd/govulncheck@latest"; rc=1
fi

echo "== Node: npm audit (web) =="
( cd apps/web && npm audit --audit-level=high ) || rc=1

echo "== Python: pip-audit (gerbonara) =="
if command -v pip-audit >/dev/null 2>&1; then
  pip-audit -r sidecar/gerbonara/requirements.txt || rc=1
else
  echo "pip-audit not installed: pipx install pip-audit"; rc=1
fi

echo "== done (rc=$rc) =="
exit $rc

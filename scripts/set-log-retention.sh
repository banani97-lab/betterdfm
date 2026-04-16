#!/usr/bin/env bash
# set-log-retention.sh
#
# Sets a retention policy on every CloudWatch Logs group used by the RapidDFM
# stack. CloudWatch Logs defaults to "Never expire" for new log groups, so
# without this the ECS/App Runner stacks accumulate log storage indefinitely.
#
# Usage:
#   AWS_PROFILE=prod AWS_REGION=us-east-1 bash scripts/set-log-retention.sh
#
# Optional first arg overrides the retention window (default 7 days). Allowed
# values are documented at:
#   https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutRetentionPolicy.html
# (1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653)

set -euo pipefail

RETENTION_DAYS="${1:-7}"
REGION="${AWS_REGION:-us-east-1}"

echo "Setting ${RETENTION_DAYS}-day retention in region ${REGION}…"

# Exact-match log groups declared in ECS task definitions
exact_groups=(
  "/ecs/betterdfm-gerbonara"
  "/ecs/betterdfm-worker"
)

# Prefix-match groups (App Runner auto-creates one per service instance with
# a random-ish suffix).
prefix_groups=(
  "/aws/apprunner/betterdfm-api"
)

apply() {
  local name="$1"
  echo "  → ${name}"
  aws logs put-retention-policy \
    --region "$REGION" \
    --log-group-name "$name" \
    --retention-in-days "$RETENTION_DAYS" \
    || echo "    (skipped; log group may not exist yet)"
}

for g in "${exact_groups[@]}"; do
  apply "$g"
done

for prefix in "${prefix_groups[@]}"; do
  groups=$(aws logs describe-log-groups \
    --region "$REGION" \
    --log-group-name-prefix "$prefix" \
    --query 'logGroups[].logGroupName' \
    --output text)
  for g in $groups; do
    apply "$g"
  done
done

echo "Done. Retention applied to existing groups."
echo "Note: ECS/App Runner may auto-recreate log groups on redeploy; this script is idempotent, re-run after infra changes."

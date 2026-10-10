#!/usr/bin/env bash
# Runs migrate-billing-org against an environment with its credentials read
# from SSM (never printed). Needs the operator's AWS profile.
#   run.sh prod            dry run
#   run.sh prod -apply     real run, with -report-levels-all
set -uo pipefail
env=${1:?usage: run.sh ENV [-apply]}
export AWS_PROFILE=${AWS_PROFILE:-ctech}
export AWS_REGION=${AWS_REGION:-$(aws configure get region)}
case "$env" in
  prod) suffix="" ;;
  *) suffix="-$env" ;;
esac
export CTECH_URL=${CTECH_URL:-https://accounts${suffix}-api.aoctech.app}
export BILLING_API_URL=${BILLING_API_URL:-https://billing${suffix}-api.aoctech.app}
g() { aws ssm get-parameter --with-decryption --name "/ctech-dfe/$env/$1" --query Parameter.Value --output text; }
export BILLING_CLIENT_ID=$(g billing/client-id)
export BILLING_CLIENT_SECRET=$(g billing/client-secret)
export ACCOUNT_WORKSPACE_CLIENT_ID=$(g account-workspace-client-id)
export ACCOUNT_WORKSPACE_CLIENT_SECRET=$(g account-workspace-client-secret)
export ACCOUNT_CLIENT_ID=$(g account-client-id)
export ACCOUNT_CLIENT_SECRET=$(g account-client-secret)
cd "$(dirname "$0")/../.."
# Built, not `go run`: go run turns the tool's exit 3 (review list) into 1.
bin=$(mktemp -d)/migrate-billing-org
go build -o "$bin" ./cmd/migrate-billing-org || exit 1
if [[ "${2:-}" == "-apply" ]]; then
  "$bin" -table-prefix "${env}_dfe" -report-levels-all -apply
else
  "$bin" -table-prefix "${env}_dfe"
fi
rc=$?
echo "exit=$rc"
exit $rc

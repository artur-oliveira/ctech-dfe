#!/bin/bash
# Started by systemd (app.service). Reads the secrets the instance is allowed to
# see, then exec-replaces itself with the Go binary so systemd supervises the app
# and not this shell.
#
# The SSM parameter *names* come from /etc/bootstrap.env, written by user data —
# they are the only part of this file that varies per environment, which is what
# lets the file itself be a static asset.
set -a
. /etc/bootstrap.env
set +a

# APP_VERSION is shipped inside the release artifact (release.env) by CI/CD.
# Format: YYMMDDHHMM:<7-char commit>. Used for the health check and nota verProc.
if [ -f /opt/app/current/release.env ]; then set -a; . /opt/app/current/release.env; set +a; fi

ssm() {
  # Every read is best-effort: a parameter that does not exist yet must leave the
  # variable empty rather than abort the boot. Each consumer decides what an empty
  # value means — see BILLING_WEBHOOK_SECRET below.
  aws ssm get-parameter --name "$1" ${2:-} --query Parameter.Value --output text --region "$AWS_REGION" 2>/dev/null || echo ""
}

# VALKEY_URL is written by the Valkey instance at boot. If the instance is scaled
# to 0 or not deployed, the parameter may not exist — fall back to empty so the
# app uses NoCacheBackend instead of crashing.
VALKEY_URL=$(ssm "$VALKEY_URL_PARAM" --with-decryption)
CTECH_JWKS_URL=$(ssm "$CTECH_JWKS_URL_PARAM")
CTECH_URL=$(ssm "$CTECH_URL_PARAM")
CTECH_ISSUER_URL=$(ssm "$CTECH_ISSUER_URL_PARAM")
SERVICE_AUDIENCE=$(ssm "$APP_URL_PARAM")

# Empty when the parameter is absent, exactly like the reads above. The webhook
# route must treat an empty secret as fatal and refuse to mount: a signature check
# that cannot run is not a signature check, and the route it guards accepts
# subscription state changes from the outside.
BILLING_WEBHOOK_SECRET=$(ssm "$BILLING_WEBHOOK_SECRET_PARAM" --with-decryption)
BILLING_API_URL=$(ssm "$BILLING_API_URL_PARAM")
BILLING_CLIENT_ID=$(ssm "$BILLING_CLIENT_ID_PARAM" --with-decryption)
BILLING_CLIENT_SECRET=$(ssm "$BILLING_CLIENT_SECRET_PARAM" --with-decryption)

CORS_ALLOWED_ORIGINS="$SERVICE_AUDIENCE"

export CTECH_JWKS_URL CTECH_URL VALKEY_URL CTECH_ISSUER_URL SERVICE_AUDIENCE CORS_ALLOWED_ORIGINS
export BILLING_WEBHOOK_SECRET BILLING_API_URL BILLING_CLIENT_ID BILLING_CLIENT_SECRET

exec /opt/app/current/app

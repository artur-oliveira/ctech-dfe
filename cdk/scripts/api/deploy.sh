#!/bin/bash
# Called by SSM RunCommand from GitHub Actions with the release key as $1.
# Expects a zip containing a pre-built `app` binary (built for linux/arm64).
set -euo pipefail
. /etc/bootstrap.env

S3_KEY="$1"
RELEASE_DIR="/opt/app/releases/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RELEASE_DIR"
echo "Downloading release: $S3_KEY"
aws s3 cp "s3://${DEPLOYMENTS_BUCKET}/$S3_KEY" /tmp/release.zip
unzip -o /tmp/release.zip -d "$RELEASE_DIR"
chmod +x "$RELEASE_DIR/app"
chown -R webapp:webapp "$RELEASE_DIR"
ln -sfT "$RELEASE_DIR" /opt/app/current
systemctl restart app 2>/dev/null || systemctl start app

for i in {1..60}; do
  if curl -sf http://127.0.0.1:8080/v1.0/health-check >/dev/null; then
    echo "Health check passed"
    break
  fi
  if systemctl is-failed --quiet app; then
    echo "Application failed to start"
    journalctl -u app --no-pager -n 100 || true
    exit 1
  fi
  sleep 2
done

curl -sf http://127.0.0.1:8080/v1.0/health-check >/dev/null || {
  echo "Timed out waiting for health check"
  exit 1
}

# Keep only the release that is live; the symlink already points at it.
ls -dt /opt/app/releases/*/ 2>/dev/null | tail -n +2 | xargs rm -rf 2>/dev/null || true
echo "Deployment successful"

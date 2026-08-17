#!/bin/bash
# Bundles the logs logrotate just rotated and ships them to S3. Called from the
# logrotate postrotate hook, so it must never fail the rotation — every step
# exits 0 on trouble.
. /etc/bootstrap.env

# IMDSv2 token required (requireImdsv2 is enforced on this instance).
TOKEN=$(curl -sf -X PUT "http://169.254.169.254/latest/api/token" \
    -H "X-aws-ec2-metadata-token-ttl-seconds: 60")
INSTANCE_ID=$(curl -sf -H "X-aws-ec2-metadata-token: $TOKEN" \
    "http://169.254.169.254/latest/meta-data/instance-id" || echo "unknown")

DATE=$(date +%Y%m%d)
ARCHIVE="/tmp/${DATE}-${INSTANCE_ID}.tar.gz"
ROTATED=$(find /var/log/app /var/log/nginx -name "*-${DATE}.gz" 2>/dev/null)
[ -z "$ROTATED" ] && exit 0

tar czf "$ARCHIVE" $ROTATED 2>/dev/null || exit 0
aws s3 cp "$ARCHIVE" "s3://${LOGS_BUCKET}/ctech-dfe/${DATE}-${INSTANCE_ID}.tar.gz" --region "$AWS_REGION" || exit 0
find /var/log/app /var/log/nginx -name "*-${DATE}.gz" -delete
rm -f "$ARCHIVE"

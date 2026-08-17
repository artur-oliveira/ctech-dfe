#!/bin/bash
# Installs the static half of the instance: nginx's configuration, the systemd
# unit, and the three operational scripts.
#
# Everything here is byte-identical across environments. What varies lives in
# /etc/bootstrap.env, written by user data before this runs — which is the whole
# point of the split: EC2 caps user data at 16 KB, and these files together are
# most of it.
set -euo pipefail

SRC="$(cd "$(dirname "$0")" && pwd)"

test -f /etc/bootstrap.env || { echo "setup.sh: /etc/bootstrap.env is missing"; exit 1; }

install -m 0644 "$SRC/nginx.conf" /etc/nginx/nginx.conf
install -m 0644 "$SRC/logrotate.conf" /etc/logrotate.d/ctech-dfe
install -m 0644 "$SRC/app.service" /etc/systemd/system/app.service
install -m 0755 "$SRC/start.sh" /opt/app/start.sh
install -m 0755 "$SRC/deploy.sh" /opt/app/deploy.sh
install -m 0755 "$SRC/upload-logs.sh" /opt/app/upload-logs.sh

# Fail the boot here rather than serve a broken proxy: nginx exits non-zero on a
# bad config, and the ASG replacing the instance is a better outcome than an
# instance that passes EC2 health checks with no listener on 8080.
nginx -t

systemctl enable nginx
systemctl start nginx

systemctl daemon-reload
systemctl enable app

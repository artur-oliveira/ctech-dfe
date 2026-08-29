#!/usr/bin/env bash
# Layer bundler for py-dfe. Runs inside the CDK Python 3.14 bundling image.
set -euo pipefail

OUT=/asset-output
mkdir -p "$OUT/python"

pip install -r requirements.txt -t "$OUT/python" --no-cache-dir

#!/usr/bin/env bash
# Layer bundler for py-dfe. Runs INSIDE the CDK Docker bundling image
# (public.ecr.aws/sam/build-python3.14 — Amazon Linux 2023, arm64, root).
#
# WeasyPrint is pure-Python but dlopen()s native libraries at runtime
# (libpango, libpangoft2, libpangocairo, libcairo, libfontconfig,
# libharfbuzz, ...). pip installs NONE of these. We dnf-install the pango
# stack, then copy every resolved .so (recursively via ldd) into
# /asset-output/lib so it lands at /opt/lib — already on Lambda's default
# LD_LIBRARY_PATH. We also ship a font + fontconfig config (text won't
# render without at least one font).
set -euo pipefail

OUT=/asset-output
mkdir -p "$OUT/python" "$OUT/lib" "$OUT/fonts"

# 1. Python deps.
pip install -r requirements.txt -t "$OUT/python" --no-cache-dir

# 2. Native libs + a font (DejaVu — broad glyph coverage).
dnf install -y --setopt=install_weak_deps=False \
  pango cairo fontconfig dejavu-sans-fonts dejavu-serif-fonts

# 3. Copy the pango entrypoints + ALL their transitive .so deps into /opt/lib.
#    ldd resolves the full graph; we copy real files (deref symlinks) and
#    recreate the soname symlink so dlopen('libpango-1.0.so.0') resolves.
copy_lib() {
  local src="$1"
  [ -e "$src" ] || return 0
  local real base
  real=$(readlink -f "$src")
  base=$(basename "$src")          # keep the soname (e.g. libpango-1.0.so.0)
  cp -u "$real" "$OUT/lib/$(basename "$real")"
  # symlink soname -> real file name if they differ
  if [ "$base" != "$(basename "$real")" ]; then
    ln -sf "$(basename "$real")" "$OUT/lib/$base"
  fi
}

ROOTS=(libpango-1.0.so.0 libpangoft2-1.0.so.0 libpangocairo-1.0.so.0 \
       libcairo.so.2 libfontconfig.so.1 libgobject-2.0.so.0)

for name in "${ROOTS[@]}"; do
  path=$(find /usr/lib64 /lib64 -name "$name" 2>/dev/null | head -1)
  [ -n "$path" ] || { echo "MISSING root lib: $name" >&2; exit 1; }
  copy_lib "$path"
  # transitive deps from ldd
  ldd "$path" | awk '/=>/ {print $3} /ld-linux/ {print $1}' \
    | grep -E '^/' | sort -u | while read -r dep; do copy_lib "$dep"; done
done

# 4. Fonts → /opt/fonts; fontconfig config with a /tmp cache (only /tmp is
#    writable on Lambda). FONTCONFIG_PATH + XDG_CACHE_HOME are set as Lambda
#    env vars in the CDK stack.
cp /usr/share/fonts/dejavu*/*.ttf "$OUT/fonts/" 2>/dev/null || \
  find /usr/share/fonts -name 'DejaVu*.ttf' -exec cp {} "$OUT/fonts/" \;

cat > "$OUT/fonts/fonts.conf" <<'EOF'
<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "fonts.dtd">
<fontconfig>
  <dir>/opt/fonts</dir>
  <cachedir>/tmp/fontconfig</cachedir>
  <config></config>
</fontconfig>
EOF

echo "Bundled libs:"; ls "$OUT/lib"
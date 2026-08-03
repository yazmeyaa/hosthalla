#!/usr/bin/env bash
set -euo pipefail

REPO="yazmeyaa/hosthalla"
ARCHIVE_PATTERN="linux_amd64"

URL=$(curl -s https://api.github.com/repos/$REPO/releases/latest \
  | jq -r --arg pattern "$ARCHIVE_PATTERN" '.assets[] | select(.name | test($pattern)) | .browser_download_url' \
  | head -n 1)

if [ -z "$URL" ] || [ "$URL" = "null" ]; then
  echo "Could not find release asset for pattern: $ARCHIVE_PATTERN" >&2
  exit 1
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -L -o "$TMP/pkg.tar.gz" "$URL"
tar -xzf "$TMP/pkg.tar.gz" -C "$TMP"

for bin in hosthalla; do
  if [ -f "$TMP/$bin" ]; then
    sudo install -m 0755 "$TMP/$bin" "/usr/local/bin/$bin"
  fi
done

VERSION=$(hosthalla version)
echo "Installed Hosthalla v$(VERSION)"

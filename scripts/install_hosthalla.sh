#!/usr/bin/env bash
set -euo pipefail

REPO="yazmeyaa/hosthalla"
ARCHIVE_PATTERN="linux_amd64"
SERVICE_USER="hosthalla"
SERVICE_GROUP="hosthalla"
CONFIG_DIR="/etc/hosthalla"
CONFIG_PATH="$CONFIG_DIR/hosthalla.yaml"
STATE_DIR="/var/lib/hosthalla"
BIN_PATH="/usr/local/bin/hosthalla"

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

if [ ! -f "$TMP/hosthalla" ]; then
  echo "Release archive does not contain the hosthalla binary" >&2
  exit 1
fi

sudo -v

if ! getent group "$SERVICE_GROUP" >/dev/null; then
  sudo groupadd --system "$SERVICE_GROUP"
fi

if id -u "$SERVICE_USER" >/dev/null 2>&1; then
  sudo usermod -a -G "$SERVICE_GROUP" "$SERVICE_USER"
else
  sudo useradd --system \
    --gid "$SERVICE_GROUP" \
    --home-dir "$STATE_DIR" \
    --shell /usr/sbin/nologin \
    "$SERVICE_USER"
fi

sudo install -d -o root -g "$SERVICE_GROUP" -m 0750 "$CONFIG_DIR"
sudo install -d -o "$SERVICE_USER" -g "$SERVICE_GROUP" -m 0750 "$STATE_DIR"
sudo chown -R "$SERVICE_USER:$SERVICE_GROUP" "$STATE_DIR"
sudo install -o root -g root -m 0755 "$TMP/hosthalla" "$BIN_PATH"

if ! sudo test -e "$CONFIG_PATH"; then
  sudo sh -c 'umask 0027; exec /usr/local/bin/hosthalla config generate'
fi
sudo chown root:"$SERVICE_GROUP" "$CONFIG_PATH"
sudo chmod 0640 "$CONFIG_PATH"

VERSION=$("$BIN_PATH" version)
echo "Installed Hosthalla v$VERSION"
echo "Config: $CONFIG_PATH"
echo "Data: $STATE_DIR"
echo "Next: sudo -u $SERVICE_USER hosthalla bootstrap --username <username> --password <password>"

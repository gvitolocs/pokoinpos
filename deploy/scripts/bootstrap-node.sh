#!/usr/bin/env bash
set -euo pipefail

APP_USER="pokoinpos"
APP_GROUP="pokoinpos"
APP_ROOT="/opt/pokoinpos"
BIN_DIR="${APP_ROOT}/bin"
STATE_DIR="/var/lib/pokoinpos"
LOG_DIR="/var/log/pokoinpos"
ENV_DIR="/etc/pokoinpos"

if [[ $EUID -ne 0 ]]; then
  echo "Run as root."
  exit 1
fi

id -u "${APP_USER}" >/dev/null 2>&1 || useradd --system --home "${STATE_DIR}" --shell /usr/sbin/nologin "${APP_USER}"
getent group "${APP_GROUP}" >/dev/null 2>&1 || groupadd --system "${APP_GROUP}"
usermod -g "${APP_GROUP}" "${APP_USER}"

install -d -m 0750 -o "${APP_USER}" -g "${APP_GROUP}" "${APP_ROOT}" "${BIN_DIR}" "${STATE_DIR}" "${LOG_DIR}" "${ENV_DIR}"

if [[ ! -f "${ENV_DIR}/node.env" ]]; then
  cat >"${ENV_DIR}/node.env" <<'EOF'
POKOINPOS_RUN_MODE=node
POKOINPOS_LISTEN_PORT=43000
POKOINPOS_JOIN_HOST=127.0.0.1
POKOINPOS_JOIN_PORT=-1
POKOINPOS_OPS_ADDR=:8080
POKOINPOS_ADMIN_TOKEN=CHANGE_ME_LONG_RANDOM_TOKEN
POKOINPOS_SLOT_SECONDS=1
POKOINPOS_GENESIS_HARDNESS=10000
POKOINPOS_GENESIS_SEED=42
POKOINPOS_INITIAL_BALANCE=1000000
EOF
  chmod 0640 "${ENV_DIR}/node.env"
  chown root:"${APP_GROUP}" "${ENV_DIR}/node.env"
fi

echo "Bootstrap complete."
echo "- Copy binary to ${BIN_DIR}/peer"
echo "- Install service: cp deploy/systemd/pokoinpos-node.service /etc/systemd/system/"
echo "- Run: systemctl daemon-reload && systemctl enable --now pokoinpos-node"

#!/usr/bin/env bash
set -euo pipefail

CERT_DIR="${MARKETPLACE_DB_CERTS_HOST_PATH:-/etc/pokoinpos-postgres/certs}"
COMMON_NAME="${MARKETPLACE_DB_TLS_COMMON_NAME:-pokoin-marketplace-postgres}"

if [[ $EUID -ne 0 ]]; then
  echo "Run as root so certificate permissions can be locked down." >&2
  exit 1
fi

mkdir -p "${CERT_DIR}"

if [[ -f "${CERT_DIR}/server.crt" && -f "${CERT_DIR}/server.key" ]]; then
  echo "TLS files already exist in ${CERT_DIR}"
  exit 0
fi

openssl req -new -x509 -days 825 -nodes \
  -subj "/CN=${COMMON_NAME}" \
  -out "${CERT_DIR}/server.crt" \
  -keyout "${CERT_DIR}/server.key"

chown -R 70:70 "${CERT_DIR}"
chmod 700 "${CERT_DIR}"
chmod 600 "${CERT_DIR}/server.key"
chmod 644 "${CERT_DIR}/server.crt"

echo "Postgres TLS certificate created in ${CERT_DIR}"

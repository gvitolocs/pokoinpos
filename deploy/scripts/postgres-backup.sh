#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-}"
if [[ -z "${ENV_FILE}" || ! -f "${ENV_FILE}" ]]; then
  echo "Usage: $0 <env-file>" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

BACKUP_DIR="${MARKETPLACE_DB_BACKUP_HOST_PATH:-/var/backups/pokoinpos-postgres}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
PLAIN_PATH="${BACKUP_DIR}/pokoin-marketplace-${TIMESTAMP}.dump"
ENCRYPTED_PATH="${PLAIN_PATH}.enc"
CONTAINER="${MARKETPLACE_DB_CONTAINER_NAME:-pokoin-marketplace-postgres}"
DATABASE="${MARKETPLACE_DB_NAME:-pokoin_marketplace}"
USER="${MARKETPLACE_DB_USER:-pokoin_marketplace}"

OCI_BUCKET="${OCI_BUCKET:-}"
OCI_NAMESPACE="${OCI_NAMESPACE:-}"
BACKUP_PASSPHRASE="${BACKUP_PASSPHRASE:-}"

if [[ -z "${BACKUP_PASSPHRASE}" ]]; then
  echo "BACKUP_PASSPHRASE is required." >&2
  exit 1
fi

mkdir -p "${BACKUP_DIR}"

docker exec "${CONTAINER}" pg_dump \
  --format=custom \
  --no-owner \
  --no-acl \
  --username="${USER}" \
  "${DATABASE}" >"${PLAIN_PATH}"

openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt \
  -in "${PLAIN_PATH}" \
  -out "${ENCRYPTED_PATH}" \
  -pass env:BACKUP_PASSPHRASE

rm -f "${PLAIN_PATH}"

if [[ -n "${OCI_BUCKET}" && -n "${OCI_NAMESPACE}" ]]; then
  oci os object put \
    --namespace-name "${OCI_NAMESPACE}" \
    --bucket-name "${OCI_BUCKET}" \
    --name "$(basename "${ENCRYPTED_PATH}")" \
    --file "${ENCRYPTED_PATH}" \
    --force
fi

echo "Postgres backup completed: ${ENCRYPTED_PATH}"

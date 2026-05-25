#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <env-file> <backup-file-or-object-name>" >&2
  exit 1
fi

ENV_FILE="$1"
BACKUP_INPUT="$2"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Env file not found: ${ENV_FILE}" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

BACKUP_PASSPHRASE="${BACKUP_PASSPHRASE:-}"
OCI_BUCKET="${OCI_BUCKET:-}"
OCI_NAMESPACE="${OCI_NAMESPACE:-}"
CONTAINER="${MARKETPLACE_DB_CONTAINER_NAME:-pokoin-marketplace-postgres}"
DATABASE="${MARKETPLACE_DB_NAME:-pokoin_marketplace}"
USER="${MARKETPLACE_DB_USER:-pokoin_marketplace}"

if [[ -z "${BACKUP_PASSPHRASE}" ]]; then
  echo "BACKUP_PASSPHRASE is required." >&2
  exit 1
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

LOCAL_ENC="${WORK_DIR}/backup.dump.enc"
LOCAL_DUMP="${WORK_DIR}/backup.dump"

if [[ -f "${BACKUP_INPUT}" ]]; then
  cp "${BACKUP_INPUT}" "${LOCAL_ENC}"
else
  if [[ -z "${OCI_BUCKET}" || -z "${OCI_NAMESPACE}" ]]; then
    echo "If backup is remote, OCI_BUCKET and OCI_NAMESPACE are required." >&2
    exit 1
  fi
  oci os object get \
    --namespace-name "${OCI_NAMESPACE}" \
    --bucket-name "${OCI_BUCKET}" \
    --name "${BACKUP_INPUT}" \
    --file "${LOCAL_ENC}"
fi

openssl enc -d -aes-256-cbc -pbkdf2 -iter 200000 \
  -in "${LOCAL_ENC}" \
  -out "${LOCAL_DUMP}" \
  -pass env:BACKUP_PASSPHRASE

docker exec -i "${CONTAINER}" pg_restore \
  --clean \
  --if-exists \
  --no-owner \
  --no-acl \
  --username="${USER}" \
  --dbname="${DATABASE}" <"${LOCAL_DUMP}"

echo "Postgres restore completed."

#!/usr/bin/env bash
set -euo pipefail

STATE_DIR="${STATE_DIR:-/var/lib/pokoinpos}"
CONFIG_FILE="${CONFIG_FILE:-/etc/pokoinpos/node.env}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/pokoinpos}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARCHIVE_PATH="${BACKUP_DIR}/pokoinpos-${TIMESTAMP}.tar.gz"
ENCRYPTED_PATH="${ARCHIVE_PATH}.enc"

OCI_BUCKET="${OCI_BUCKET:-}"
OCI_NAMESPACE="${OCI_NAMESPACE:-}"
BACKUP_PASSPHRASE="${BACKUP_PASSPHRASE:-}"

if [[ -z "${BACKUP_PASSPHRASE}" ]]; then
  echo "BACKUP_PASSPHRASE is required."
  exit 1
fi

mkdir -p "${BACKUP_DIR}"
tar -czf "${ARCHIVE_PATH}" "${STATE_DIR}" "${CONFIG_FILE}"

openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt \
  -in "${ARCHIVE_PATH}" \
  -out "${ENCRYPTED_PATH}" \
  -pass env:BACKUP_PASSPHRASE

rm -f "${ARCHIVE_PATH}"

if [[ -n "${OCI_BUCKET}" && -n "${OCI_NAMESPACE}" ]]; then
  oci os object put \
    --namespace-name "${OCI_NAMESPACE}" \
    --bucket-name "${OCI_BUCKET}" \
    --name "$(basename "${ENCRYPTED_PATH}")" \
    --file "${ENCRYPTED_PATH}" \
    --force
fi

echo "Backup completed: ${ENCRYPTED_PATH}"

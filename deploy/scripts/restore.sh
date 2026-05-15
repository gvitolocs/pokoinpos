#!/usr/bin/env bash
set -euo pipefail

BACKUP_INPUT="${1:-}"
RESTORE_ROOT="${RESTORE_ROOT:-/}"
BACKUP_PASSPHRASE="${BACKUP_PASSPHRASE:-}"
OCI_BUCKET="${OCI_BUCKET:-}"
OCI_NAMESPACE="${OCI_NAMESPACE:-}"

if [[ -z "${BACKUP_INPUT}" ]]; then
  echo "Usage: $0 <backup-file-or-object-name>"
  exit 1
fi

if [[ -z "${BACKUP_PASSPHRASE}" ]]; then
  echo "BACKUP_PASSPHRASE is required."
  exit 1
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

LOCAL_ENC="${WORK_DIR}/backup.enc"
LOCAL_TAR="${WORK_DIR}/backup.tar.gz"

if [[ -f "${BACKUP_INPUT}" ]]; then
  cp "${BACKUP_INPUT}" "${LOCAL_ENC}"
else
  if [[ -z "${OCI_BUCKET}" || -z "${OCI_NAMESPACE}" ]]; then
    echo "If backup is remote, OCI_BUCKET and OCI_NAMESPACE are required."
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
  -out "${LOCAL_TAR}" \
  -pass env:BACKUP_PASSPHRASE

tar -xzf "${LOCAL_TAR}" -C "${RESTORE_ROOT}"
echo "Restore completed into ${RESTORE_ROOT}"

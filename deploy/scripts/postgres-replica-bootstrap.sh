#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 || "${2:-}" != "--confirm-wipe-replica-data" ]]; then
  echo "Usage: $0 <replica-env-file> --confirm-wipe-replica-data" >&2
  echo "This wipes MARKETPLACE_DB_DATA_HOST_PATH before running pg_basebackup." >&2
  exit 1
fi

ENV_FILE="$1"
if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Env file not found: ${ENV_FILE}" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

CONTAINER="${MARKETPLACE_DB_CONTAINER_NAME:-pokoin-marketplace-postgres-replica}"
DATA_DIR="${MARKETPLACE_DB_DATA_HOST_PATH:-/var/lib/pokoinpos-postgres-replica}"
PRIMARY_HOST="${MARKETPLACE_DB_PRIMARY_HOST:-}"
PRIMARY_PORT="${MARKETPLACE_DB_PRIMARY_PORT:-5432}"
REPLICATION_USER="${MARKETPLACE_DB_REPLICATION_USER:-pokoin_replicator}"
REPLICATION_PASSWORD="${MARKETPLACE_DB_REPLICATION_PASSWORD:-}"
REPLICATION_SLOT="${MARKETPLACE_DB_REPLICATION_SLOT:-pokoin_peer3_replica}"
TEMP_CONTAINER="${CONTAINER}-basebackup"

if [[ -z "${PRIMARY_HOST}" ]]; then
  echo "MARKETPLACE_DB_PRIMARY_HOST is required." >&2
  exit 1
fi

if [[ -z "${REPLICATION_PASSWORD}" ]]; then
  echo "MARKETPLACE_DB_REPLICATION_PASSWORD is required." >&2
  exit 1
fi

if [[ "${DATA_DIR}" == "/" || "${DATA_DIR}" == "/var" || "${DATA_DIR}" == "/var/lib" ]]; then
  echo "Refusing unsafe data directory: ${DATA_DIR}" >&2
  exit 1
fi

if docker ps --format '{{.Names}}' | grep -Fxq "${CONTAINER}"; then
  echo "Replica container is running; stop it before bootstrapping: ${CONTAINER}" >&2
  exit 1
fi

if [[ -f "${DATA_DIR}/pgdata/postmaster.pid" ]]; then
  echo "Refusing to wipe active-looking Postgres data directory: ${DATA_DIR}" >&2
  exit 1
fi

mkdir -p "${DATA_DIR}"
rm -rf "${DATA_DIR:?}/"*

docker rm -f "${TEMP_CONTAINER}" >/dev/null 2>&1 || true
docker run --rm \
  --name "${TEMP_CONTAINER}" \
  -e "PGPASSWORD=${REPLICATION_PASSWORD}" \
  -v "${DATA_DIR}:/var/lib/postgresql/data" \
  postgres:17-alpine \
  pg_basebackup \
    --host="${PRIMARY_HOST}" \
    --port="${PRIMARY_PORT}" \
    --username="${REPLICATION_USER}" \
    --pgdata=/var/lib/postgresql/data/pgdata \
    --format=plain \
    --write-recovery-conf \
    --slot="${REPLICATION_SLOT}" \
    --create-slot \
    --checkpoint=fast \
    --progress \
    --verbose

echo "Replica base backup completed in ${DATA_DIR}."

#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <replica-env-file>" >&2
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
DATABASE="${MARKETPLACE_DB_NAME:-pokoin_marketplace}"
USER="${MARKETPLACE_DB_USER:-pokoin_marketplace}"
PASSWORD="${MARKETPLACE_DB_PASSWORD:-}"
PUBLIC_HOST="${MARKETPLACE_DB_PUBLIC_HOST:-}"
PORT="${MARKETPLACE_DB_PORT:-5432}"

if [[ -z "${PASSWORD}" ]]; then
  echo "MARKETPLACE_DB_PASSWORD is required so the failover URL can be printed." >&2
  exit 1
fi
if [[ -z "${PUBLIC_HOST}" ]]; then
  echo "MARKETPLACE_DB_PUBLIC_HOST is required so the failover URL can be printed." >&2
  exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -Fxq "${CONTAINER}"; then
  echo "Container is not running: ${CONTAINER}" >&2
  exit 1
fi

is_replica="$(docker exec "${CONTAINER}" psql \
  --username="${USER}" \
  --dbname="${DATABASE}" \
  --tuples-only \
  --no-align \
  --set=ON_ERROR_STOP=1 \
  --command="select pg_is_in_recovery();")"

if [[ "${is_replica}" != "t" ]]; then
  echo "Postgres is already writable or not in recovery; no promotion needed." >&2
else
  docker exec "${CONTAINER}" psql \
    --username="${USER}" \
    --dbname="${DATABASE}" \
    --set=ON_ERROR_STOP=1 \
    --command="select pg_promote(wait => true, wait_seconds => 60);"
fi

docker exec "${CONTAINER}" psql \
  --username="${USER}" \
  --dbname="${DATABASE}" \
  --set=ON_ERROR_STOP=1 \
  --command="create table if not exists public.failover_write_probe (checked_at timestamptz primary key default now()); insert into public.failover_write_probe default values;"

echo "Peer3 Postgres promoted and verified writable."
echo
echo "Set Vercel/API MARKETPLACE_DATABASE_URL to:"
echo "postgresql://${USER}:${PASSWORD}@${PUBLIC_HOST}:${PORT}/${DATABASE}?sslmode=require"
echo
echo "Do not restart old peer4 as primary until it has been re-seeded from this promoted database."

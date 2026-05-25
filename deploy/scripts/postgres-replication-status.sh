#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <primary-env-file> <replica-env-file>" >&2
  exit 1
fi

PRIMARY_ENV="$1"
REPLICA_ENV="$2"

if [[ ! -f "${PRIMARY_ENV}" ]]; then
  echo "Primary env file not found: ${PRIMARY_ENV}" >&2
  exit 1
fi
if [[ ! -f "${REPLICA_ENV}" ]]; then
  echo "Replica env file not found: ${REPLICA_ENV}" >&2
  exit 1
fi

load_env() {
  local env_file="$1"
  set -a
  # shellcheck disable=SC1090
  source "${env_file}"
  set +a
}

load_env "${PRIMARY_ENV}"
PRIMARY_CONTAINER="${MARKETPLACE_DB_CONTAINER_NAME:-pokoin-marketplace-postgres}"
PRIMARY_DATABASE="${MARKETPLACE_DB_NAME:-pokoin_marketplace}"
PRIMARY_USER="${MARKETPLACE_DB_USER:-pokoin_marketplace}"

unset MARKETPLACE_DB_CONTAINER_NAME MARKETPLACE_DB_NAME MARKETPLACE_DB_USER
load_env "${REPLICA_ENV}"
REPLICA_CONTAINER="${MARKETPLACE_DB_CONTAINER_NAME:-pokoin-marketplace-postgres-replica}"
REPLICA_DATABASE="${MARKETPLACE_DB_NAME:-pokoin_marketplace}"
REPLICA_USER="${MARKETPLACE_DB_USER:-pokoin_marketplace}"

echo "Primary replication senders:"
docker exec "${PRIMARY_CONTAINER}" psql \
  --username="${PRIMARY_USER}" \
  --dbname="${PRIMARY_DATABASE}" \
  --set=ON_ERROR_STOP=1 \
  --command="select application_name, client_addr, state, sync_state, sent_lsn, write_lsn, flush_lsn, replay_lsn, write_lag, flush_lag, replay_lag from pg_stat_replication order by application_name, client_addr;"

echo
echo "Primary replication slots:"
docker exec "${PRIMARY_CONTAINER}" psql \
  --username="${PRIMARY_USER}" \
  --dbname="${PRIMARY_DATABASE}" \
  --set=ON_ERROR_STOP=1 \
  --command="select slot_name, active, restart_lsn, confirmed_flush_lsn, wal_status, safe_wal_size from pg_replication_slots order by slot_name;"

echo
echo "Replica recovery status:"
docker exec "${REPLICA_CONTAINER}" psql \
  --username="${REPLICA_USER}" \
  --dbname="${REPLICA_DATABASE}" \
  --set=ON_ERROR_STOP=1 \
  --command="select pg_is_in_recovery() as is_replica, pg_last_wal_receive_lsn() as receive_lsn, pg_last_wal_replay_lsn() as replay_lsn, now() - pg_last_xact_replay_timestamp() as replay_delay;"

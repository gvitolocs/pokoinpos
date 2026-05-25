#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <primary-env-file>" >&2
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

CONTAINER="${MARKETPLACE_DB_CONTAINER_NAME:-pokoin-marketplace-postgres}"
DATABASE="${MARKETPLACE_DB_NAME:-pokoin_marketplace}"
ADMIN_USER="${MARKETPLACE_DB_USER:-pokoin_marketplace}"
REPLICATION_USER="${MARKETPLACE_DB_REPLICATION_USER:-pokoin_replicator}"
REPLICATION_PASSWORD="${MARKETPLACE_DB_REPLICATION_PASSWORD:-}"
REPLICA_CIDRS="${MARKETPLACE_DB_REPLICA_CIDRS:-}"

if [[ -z "${REPLICATION_PASSWORD}" ]]; then
  echo "MARKETPLACE_DB_REPLICATION_PASSWORD is required." >&2
  exit 1
fi

if [[ -z "${REPLICA_CIDRS}" ]]; then
  echo "MARKETPLACE_DB_REPLICA_CIDRS is required; refuse to allow open replication." >&2
  exit 1
fi

if [[ "${REPLICA_CIDRS}" == *"0.0.0.0/0"* || "${REPLICA_CIDRS}" == *"::/0"* ]]; then
  echo "Refusing open replication CIDR in MARKETPLACE_DB_REPLICA_CIDRS." >&2
  exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -Fxq "${CONTAINER}"; then
  echo "Container is not running: ${CONTAINER}" >&2
  exit 1
fi

function escape_sql_literal {
  printf "%s" "$1" | sed "s/'/''/g"
}

escaped_user="$(escape_sql_literal "${REPLICATION_USER}")"
escaped_password="$(escape_sql_literal "${REPLICATION_PASSWORD}")"

docker exec -i "${CONTAINER}" psql \
  --username="${ADMIN_USER}" \
  --dbname="${DATABASE}" \
  --set=ON_ERROR_STOP=1 <<SQL
do \$\$
begin
  if not exists (select 1 from pg_roles where rolname = '${escaped_user}') then
    execute format('create role %I with replication login password %L', '${escaped_user}', '${escaped_password}');
  else
    execute format('alter role %I with replication login password %L', '${escaped_user}', '${escaped_password}');
  end if;
end
\$\$;
SQL

IFS=',' read -r -a cidr_array <<<"${REPLICA_CIDRS}"
for cidr in "${cidr_array[@]}"; do
  trimmed="$(echo "${cidr}" | xargs)"
  [[ -n "${trimmed}" ]] || continue
  rule="hostssl replication ${REPLICATION_USER} ${trimmed} scram-sha-256"
  docker exec "${CONTAINER}" sh -c \
    "grep -Fxq '$rule' \"\$PGDATA/pg_hba.conf\" || printf '%s\n' '$rule' >> \"\$PGDATA/pg_hba.conf\""
done

docker exec "${CONTAINER}" psql \
  --username="${ADMIN_USER}" \
  --dbname="${DATABASE}" \
  --set=ON_ERROR_STOP=1 \
  --command="select pg_reload_conf();"

echo "Postgres primary replication configured for ${REPLICATION_USER}."

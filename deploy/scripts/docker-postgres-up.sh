#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <env-file>" >&2
  echo "Example: $0 deploy/env/peer4-postgres.env" >&2
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

if [[ $EUID -eq 0 ]]; then
  deploy/scripts/postgres-init-tls.sh
else
  echo "Skipping TLS initialization because current user is not root."
  echo "Run deploy/scripts/postgres-init-tls.sh as root before first start."
fi

docker compose --env-file "${ENV_FILE}" -f docker-compose.peer4-postgres.yml pull
docker compose --env-file "${ENV_FILE}" -f docker-compose.peer4-postgres.yml up -d --remove-orphans

echo "Oracle marketplace Postgres started."

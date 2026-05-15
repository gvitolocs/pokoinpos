#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <env-file>"
  echo "Example: $0 deploy/env/peer2.env"
  exit 1
fi

ENV_FILE="$1"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Env file not found: ${ENV_FILE}"
  exit 1
fi

docker compose --env-file "${ENV_FILE}" -f docker-compose.peer.yml pull
docker compose --env-file "${ENV_FILE}" -f docker-compose.peer.yml up -d --remove-orphans

echo "Peer started with auto-update service enabled."

#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <env-file>" >&2
  exit 1
fi

if [[ $EUID -ne 0 ]]; then
  echo "Run as root." >&2
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

PORT="${MARKETPLACE_DB_PORT:-5432}"
CIDRS="${MARKETPLACE_DB_ALLOWED_CIDRS:-}"

if [[ -z "${CIDRS}" ]]; then
  echo "MARKETPLACE_DB_ALLOWED_CIDRS is empty; refusing to open ${PORT}/tcp." >&2
  exit 1
fi

IFS=',' read -r -a cidr_array <<<"${CIDRS}"
for cidr in "${cidr_array[@]}"; do
  trimmed="$(echo "${cidr}" | xargs)"
  [[ -n "${trimmed}" ]] || continue
  ufw allow from "${trimmed}" to any port "${PORT}" proto tcp
done

ufw reload
ufw status numbered

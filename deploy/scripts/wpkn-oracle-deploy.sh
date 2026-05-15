#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-/opt/pokoinpos}"
SECRETS_FILE="${WPKN_SECRETS_FILE:-/opt/pokoinpos-secrets/wpkn.env}"
CONTRACTS_DIR="${REPO_ROOT}/contracts"

if [[ ! -f "${SECRETS_FILE}" ]]; then
  echo "Missing secrets file: ${SECRETS_FILE}" >&2
  echo "Create it from contracts/.env.example and fund the deployer wallet with BNB first." >&2
  exit 1
fi

if [[ ! -d "${CONTRACTS_DIR}" ]]; then
  echo "Missing contracts workspace: ${CONTRACTS_DIR}" >&2
  exit 1
fi

if [[ "$(stat -c '%a' "${SECRETS_FILE}")" != "600" ]]; then
  echo "Refusing to use ${SECRETS_FILE}: permissions must be 600." >&2
  exit 1
fi

cd "${CONTRACTS_DIR}"
npm ci

set -a
# shellcheck disable=SC1090
. "${SECRETS_FILE}"
set +a

npm run build
npm run deploy:bnb

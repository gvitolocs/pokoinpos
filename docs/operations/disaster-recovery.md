# Disaster Recovery Runbook (Oracle Free Tier MVP)

This runbook defines backup, restore, and validation for the single-node permissioned PoS MVP.

## Scope

- Node runtime state: `/var/lib/pokoinpos`
- Node runtime configuration: `/etc/pokoinpos/node.env`
- Optional offsite storage: OCI Object Storage

## RPO and RTO targets

- **RPO target**: 24 hours (daily encrypted backup)
- **RTO target**: 30 minutes (new VM bootstrap + restore + health check)

## Backup procedure

1. Set required environment variables:
   - `BACKUP_PASSPHRASE`
   - optional: `OCI_NAMESPACE`, `OCI_BUCKET`
2. Run:
   - `deploy/scripts/backup.sh`
3. Confirm artifact exists:
   - local: `/var/backups/pokoinpos/pokoinpos-<timestamp>.tar.gz.enc`
   - remote: object present in OCI bucket if configured

## Restore procedure (new VM)

1. Provision VM in Oracle Free Tier home region.
2. Apply host baseline:
   - `deploy/scripts/hardening.sh`
   - `deploy/scripts/bootstrap-node.sh`
3. Stop node service if running:
   - `systemctl stop pokoinpos-node`
4. Set passphrase:
   - `export BACKUP_PASSPHRASE=...`
5. Restore from local file:
   - `deploy/scripts/restore.sh /path/to/pokoinpos-<timestamp>.tar.gz.enc`
6. Or restore from OCI Object Storage object:
   - `export OCI_NAMESPACE=...`
   - `export OCI_BUCKET=...`
   - `deploy/scripts/restore.sh pokoinpos-<timestamp>.tar.gz.enc`
7. Restart service:
   - `systemctl start pokoinpos-node`
8. Validate:
   - `curl -s http://127.0.0.1:8080/health`
   - `curl -s http://127.0.0.1:8080/ready`
   - `curl -s http://127.0.0.1:8080/chain/status`

## DR test cadence (GameDay)

- Execute full restore drill at least monthly.
- Record:
  - start/end time
  - RTO achieved
  - backup timestamp and resulting RPO
  - validation outcomes for health/ready/chain endpoints
- Open a remediation ticket if RTO > 30 minutes or validation fails.

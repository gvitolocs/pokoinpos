# Peer3 Marketplace Postgres Fallback

This runbook makes peer3 a hot standby fallback for the marketplace Postgres
database currently written on peer4.

## Topology

- Peer4 is the only writable primary.
- Peer3 is a read-only PostgreSQL physical streaming replica and the manual
  failover target.
- Peer2 and peer1 may also run read-only physical streaming replicas for
  CardVault variation/search-dimension reads. They are not failover targets in
  this runbook.
- Vercel marketplace writes continue using peer4 until a manual failover.
- Vercel marketplace autocomplete may use peer3 for card-name-only matching
  through `MARKETPLACE_NAME_SEARCH_DATABASE_URL`.
- Encrypted backups remain mandatory.

Do not run peer4 and peer3 as writable primaries at the same time.

## Environment Files

PokoinPoS and CardVault may both keep local copies of Oracle key material and
database env files because both projects operate Oracle services. Keep
`deploy/env/*.env`, `.env.local`, and `keys/` local-only and gitignored, never
print their contents, and manually synchronize copies when rotating Oracle
credentials. CardVault's copy lives under
`/Users/giuseppe/cardvault/pokemon_card_vault/` on this machine.

Primary:

```bash
cp deploy/env/peer4-postgres.env.example deploy/env/peer4-postgres.env
```

Set strong values for:

- `MARKETPLACE_DB_PASSWORD`
- `MARKETPLACE_DB_REPLICATION_PASSWORD`
- `MARKETPLACE_DB_REPLICA_CIDRS`
- `MARKETPLACE_DB_ALLOWED_CIDRS`

`MARKETPLACE_DB_REPLICA_CIDRS` should contain only known replica hosts, for
example peer3/peer2/peer1 `/32` CIDRs. Do not use `0.0.0.0/0`.

Replica:

```bash
cp deploy/env/peer3-postgres-replica.env.example deploy/env/peer3-postgres-replica.env
```

Set:

- `MARKETPLACE_DB_PRIMARY_HOST` to peer4.
- `MARKETPLACE_DB_PUBLIC_HOST` to peer3.
- `MARKETPLACE_DB_PASSWORD` to the same application database password used by
  peer4.
- `MARKETPLACE_DB_REPLICATION_PASSWORD` to the same replication password used by
  peer4.

## Primary Setup on Peer4

```bash
sudo deploy/scripts/postgres-init-tls.sh
./deploy/scripts/docker-postgres-up.sh deploy/env/peer4-postgres.env
deploy/scripts/postgres-primary-replication-setup.sh deploy/env/peer4-postgres.env
sudo deploy/scripts/postgres-firewall.sh deploy/env/peer4-postgres.env
```

The setup script creates or updates the replication role and appends restricted
`hostssl replication` entries for configured replica CIDRs to `pg_hba.conf`.

## Replica Bootstrap on Peer3

Run this only on the peer3 host. It wipes the configured replica data directory.

```bash
sudo deploy/scripts/postgres-init-tls.sh
deploy/scripts/postgres-replica-bootstrap.sh deploy/env/peer3-postgres-replica.env --confirm-wipe-replica-data
./deploy/scripts/docker-postgres-up.sh deploy/env/peer3-postgres-replica.env
sudo deploy/scripts/postgres-firewall.sh deploy/env/peer3-postgres-replica.env
```

The bootstrap script uses `pg_basebackup -R` and creates the configured
replication slot on peer4.

Peer3 must also allow external TCP `5432` at both layers:

- Host firewall: add and persist an iptables/nftables rule for `tcp dpt:5432`.
- OCI ingress: add a security-list or NSG rule for TCP `5432` to the peer3 VNIC.

If local `127.0.0.1:5432` works on peer3 but remote clients time out, check OCI
ingress first. If the OCI rule exists but packets hit the host reject rule, check
the host firewall.

## Normal Update Workflow

Apply marketplace database changes to peer4 only:

```bash
# Example from the CardVault repo:
node scripts/oracle-marketplace-migrate.js schema
node scripts/oracle-marketplace-migrate.js refresh
node scripts/oracle-marketplace-migrate.js verify
```

Then verify replication:

```bash
deploy/scripts/postgres-replication-status.sh deploy/env/peer4-postgres.env deploy/env/peer3-postgres-replica.env
```

Healthy output should show an active sender on peer4 and `pg_is_in_recovery()`
as true on peer3.

## Split Search Reads

CardVault can halve some autocomplete database work by using both hosts:

- `MARKETPLACE_DATABASE_URL`: peer4 primary, used for writes, refreshes,
  migrations, analytics events, non-name search fields, and full fallback search.
- `MARKETPLACE_NAME_SEARCH_DATABASE_URL`: peer3 standby, used only for
  card-name candidate matching.
- `MARKETPLACE_VARIATION_SEARCH_REPLICA_URLS`: optional peer2/peer1 standby
  URLs, used for variation and search-dimension fanout after those replicas are
  healthy.

If peer3 is unavailable or lagging, the API must fall back to peer4 full search.
Do not point write paths, migration scripts, or event recording at
`MARKETPLACE_NAME_SEARCH_DATABASE_URL` or
`MARKETPLACE_VARIATION_SEARCH_REPLICA_URLS`.

Production Vercel must have:

- `MARKETPLACE_DATABASE_URL`: peer4 primary.
- `MARKETPLACE_NAME_SEARCH_DATABASE_URL`: peer3 replica.
- `MARKETPLACE_NAME_SEARCH_TIMEOUT_MS`: short peer3 read timeout.
- `MARKETPLACE_NAME_SEARCH_CIRCUIT_MS`: temporary peer3 circuit breaker after a
  timeout.
- `MARKETPLACE_VARIATION_SEARCH_REPLICA_URLS`: optional comma-separated peer2
  and peer1 replica URLs after replication checks pass.
- `MARKETPLACE_VARIATION_SEARCH_TIMEOUT_MS`: short dimension replica read
  timeout.
- `MARKETPLACE_VARIATION_SEARCH_CIRCUIT_MS`: temporary peer2/peer1 circuit
  breaker after a timeout.

The peer2/peer1 setup checklist lives in CardVault:

```bash
open /Users/giuseppe/cardvault/pokemon_card_vault/workflows/oracle-marketplace-postgres-workflow.md
```

## Manual Failover

Use this only when peer4 is down or intentionally removed from service.

```bash
deploy/scripts/postgres-failover-promote.sh deploy/env/peer3-postgres-replica.env
```

After promotion:

1. Update Vercel `MARKETPLACE_DATABASE_URL` to the peer3 URL printed by the
   script.
2. Update `MARKETPLACE_NAME_SEARCH_DATABASE_URL` to either the same promoted
   peer3 URL or leave it unset until a new read replica exists.
3. Redeploy the web/API project.
4. Keep old peer4 offline or blocked from database clients.
5. Re-seed peer4 from the promoted peer3 before returning it to service.

## Rebuild Old Peer4 After Failover

After peer3 is promoted, peer4 is stale and must not be restarted as primary.
Treat peer3 as the new primary and bootstrap peer4 from it using the same
replica pattern with host names reversed.

At minimum:

1. Back up peer3.
2. Stop old peer4 Postgres.
3. Wipe only the old peer4 Postgres data directory after confirming the path.
4. Run a fresh base backup from peer3.
5. Start old peer4 as a replica or intentionally promote it during a planned
   switchover.

## Backups

Streaming replication does not protect against bad writes. Keep running:

```bash
BACKUP_PASSPHRASE=... deploy/scripts/postgres-backup.sh deploy/env/peer4-postgres.env
```

After failover, run backups against the promoted peer3 env file.

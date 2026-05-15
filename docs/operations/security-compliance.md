# Security and Compliance Baseline

This document is split into:

- **Implemented controls** for the current permissioned PoS node MVP on Oracle Cloud Free Tier.
- **Production controls roadmap** still required before regulated payment production.

## Implemented controls (current repository)

### Host hardening

- Scripted hardening in `deploy/scripts/hardening.sh`:
  - SSH keys only (`PasswordAuthentication no`, `PermitRootLogin no`)
  - `ufw` default deny inbound with explicit allow rules
  - `fail2ban` for SSH brute-force mitigation
  - unattended security upgrades enabled

### Process isolation

- Systemd service template in `deploy/systemd/pokoinpos-node.service`:
  - dedicated `pokoinpos` system user/group
  - `NoNewPrivileges=true`
  - `ProtectSystem=strict`, `ProtectHome=true`, `PrivateTmp=true`
  - constrained writable paths (`/var/lib/pokoinpos`, `/var/log/pokoinpos`)
  - high descriptor/process limits suitable for p2p workloads

### Runtime secret separation

- Runtime config loaded from env file (`/etc/pokoinpos/node.env`) via bootstrap script.
- Admin API endpoints require `Authorization: Bearer <token>` (`POKOINPOS_ADMIN_TOKEN`).
- Secrets are not hardcoded in source.

### Node health and operations surface

- Operational endpoints implemented:
  - `GET /health`
  - `GET /ready`
  - `GET /chain/status`
  - `GET /metrics` (Prometheus text format)
  - `POST /admin/mine?slot=<n>` (admin-token protected)

## Production controls roadmap (remaining)

### Key management

- Move validator signing key to HSM/KMS-backed signer adapter.
- Add key rotation and emergency key revocation runbook.
- Separate validator signing identity from operational API credentials.

### API and app security

- Enforce TLS termination with certificate rotation automation.
- Add JWT-based operator auth with short-lived tokens and revocation.
- Add rate limiting and request signing on high-risk endpoints.

### Auditability and compliance

- Immutable audit trail for admin actions and consensus events.
- Data retention policy by region/jurisdiction.
- Formal incident response process with postmortem template.

### Security testing gates

- Add dependency scanning and SBOM generation in CI.
- Add fuzz tests for p2p message parsing and signature handling.
- Add replay/equivocation test suites in CI.

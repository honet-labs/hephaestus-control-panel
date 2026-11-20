# Audit follow-up — 20 November 2026

This supplements `SECURITY-AUDIT-2026-11.md`; it does not replace its deployment
warnings. Production was not modified or penetration-tested.

## Additional verified fixes

1. **Vaultwarden cached authentication bypass:** cached sessions were indexed only
   by server/email. A different profile with an incorrect password could reuse a
   previously authenticated session. Reuse now requires a constant-time match of
   the password binding; case-sensitive endpoint paths remain distinct.
2. **Untrusted vault KDF resource exhaustion:** prelogin data could request enormous
   PBKDF2/Argon2 work or integer-truncated parallelism. Parameters are bounded and
   unsupported KDFs rejected. Missing Argon2 iterations default to 3, not 600,000.
   AES-CBC validates IV length before invoking the cipher implementation.
3. **Credential destination binding:** backup database/SSH, S3/SFTP destinations,
   OpenSearch and Vaultwarden test endpoints reject reuse against changed
   destinations/identities. OpenSearch invalid IDs no longer fall back to the
   active profile in the test handler. TLS policy cannot be weakened while reusing
   saved OpenSearch credentials. Global OpenSearch activation requires manage.
4. **Backup secret persistence:** masked destination saves preserve ciphertext
   verbatim instead of encrypting it twice. SFTP private keys are encrypted,
   decrypted for service use and masked in lists. Save responses omit secrets.
   Encryption/decryption errors fail closed.
5. **HTTP credential redirects:** vault, report, status-page, OpenSearch and Grafana
   test clients do not follow redirects to another destination. Operators must
   configure the final endpoint URL.
6. **Docker SSH wrapper:** removed automatic `usermod -aG docker` and `chmod 666`
   socket changes from ordinary Docker operations. Socket paths are POSIX shell
   quoted, including streaming backup/restore wrappers. Provision the account's
   Docker/sudo access explicitly; a read/test must not weaken host permissions.
7. **Backup integrity and file safety:** missing/incompatible `pg_dump` fails the
   backup instead of silently substituting a schema-less data dump. Remote dump
   pipelines use bash `pipefail` and private umask; without bash, use the local
   client through the existing SSH-tunnel fallback. Backup paths cannot traverse
   outside `/app/backups`; database names are sanitized for archive filenames.
   New database archive files use mode 0600 and unique filename suffixes.
8. **Migration failure:** failed initial/pre-upgrade schema execution is no longer
   logged as a harmless notice. Failed migration does not expose a connected pool.
9. **Continuity/maintenance:** wired existing monitoring/IPAM/vault background stop
   signals into shutdown; removed obsolete in-memory dump implementations and
   unused helpers/state. Replaced deprecated AWS endpoint-resolver usage with
   service-specific S3 BaseEndpoint. Removed SNMP community secrets from timeout
   messages and replaced string client-IP context keys with a typed key.

## Executed validation

- Isolated **PostgreSQL 17.12** on loopback, with a uniquely created disposable
  database: initial and repeated migrations, eight concurrent setup attempts
  (exactly one admin), five-profile owner/read/manage/public/revoked access matrix,
  HTTP resource-write denial, self/admin password session revocation, destination
  secret preservation and response masking.
- Real loopback **SSH/SFTP protocol** server: command execution, file download,
  persisted first-use enrollment and rejection after host-key rotation.
- Controlled **HTTP/TLS/S3-compatible fixtures**: no credential redirects, rejection
  of untrusted HTTPS certificates, signed memory/file S3 uploads, vault session
  binding, KDF bounds and authenticated encryption validation.
- Source-level route registration test checks feature/role guards on the protected
  API group, including compatibility aliases. This is not proof of complete
  resource authorization on every endpoint.
- Go staticcheck identified 33 diagnostics before this pass; verified unused
  private dump functions/helpers and dead stores were removed, and the remaining
  reported correctness/style/deprecation findings addressed.
- Deadcode reachability analysis was run with tests enabled. Not every unreachable
  exported method is automatically deleted: reserved APIs and unconnected
  notification functionality need a product decision. Lifecycle stop methods
  were connected rather than removed.

These fixtures are real protocols, **not** vendor deployment certification.
No customer PostgreSQL database, infrastructure account or production vault was used.

## Image CVE findings and CI evidence

The earlier commit's CI result has now been checked, not assumed:
[run 44547746291](https://github.com/honet-labs/hephaestus-control-panel/actions/runs/44547746291)
for `eb63785` passed Linux race tests, dependency scans, frontend build and Docker
backend/frontend builds for amd64/arm64.

Trivy **0.76.2** remotely scanned the amd64 base images on 20 November:

| Image | Findings | Unique CVEs | Fixed package versions available |
| --- | --- | --- | --- |
| alpine:3.24.4 | 11 medium package findings | 6 | OpenSSL 3.5.9-r1, zlib 1.4-r0 |
| nginx:1.30.7-alpine | 11 medium package findings | 6 | OpenSSL 3.5.9-r1, zlib 1.4-r0 |

The advisories are CVE-2026-107496, CVE-2026-88822, CVE-2026-88823,
CVE-2026-95691, CVE-2026-95692 and CVE-2026-4738. The OpenSSL advisories were
reported against both libcrypto3 and libssl3; do not count those as distinct CVEs.
These are **base-image** findings, not a scan of the final HCP application image.

Runtime Dockerfiles now upgrade Alpine packages before adding dependencies.
CI builds and loads each final backend/frontend image separately for amd64 and
arm64, scans OS and application dependencies with Trivy, fails on medium/high/
critical findings and uploads the JSON results. PostgreSQL integration tests run
with race detection against a disposable CI service. Final-image outcomes must be
read from that commit's CI run; adding a scan step is not evidence it passed.

## Reproduction

```sh
go test ./...
go vet ./...
staticcheck ./...
govulncheck ./...
# Explicit isolated local test instance; account needs CREATE DATABASE.
HCP_AUDIT_POSTGRES=1 HCP_AUDIT_POSTGRES_PORT=55439 \
  HCP_AUDIT_POSTGRES_USER=hcp_audit go test ./tests/integration -v -count=1
```

Only loopback is used by the database test. It creates a fresh `hcp_audit_*`
database and drops that specific database afterward. Do not provide production
database credentials. Default `go test` skips this opt-in database test.

## Upgrade actions and outstanding deployment checks

- Re-enter historical destination credentials if they were already saved in a
  double-encrypted state; this pass prevents recurrence, not automatic recovery.
- Install a compatible `pg_dump` in custom/native deployments. Previously generated
  native-fallback archives are not proven full backups; perform a restore drill.
- Private file modes apply to newly created files; review permissions of old
  archives/backups and Windows ACLs separately. POSIX modes do not enforce Windows ACLs.
- Explicitly provision Docker group/sudo permissions; investigate and correct any
  pre-existing world-writable sockets created by older versions.
- Vendor OpenSearch, Grafana, Prometheus, Data Prepper, OTel, Vaultwarden and S3
  staging endpoints, genuine database backup/restore, deployment migrations from
  actual historical customer schemas and production cross-user sessions still
  need an authorized staging environment. Loopback fixtures do not replace these.
- General control-plane risks (trusted manage roles, egress/SSRF policy, proxy
  token logging, intentionally public resources) remain as described in the first
  report. No claim of an exhaustive penetration test or zero application risk.

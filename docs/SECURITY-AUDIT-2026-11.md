# HCP security and continuity audit — November 2026

> Follow-up: see [20 November validation and additional fixes](SECURITY-AUDIT-FOLLOWUP-2026-11.md)
> for live PostgreSQL/SSH/SFTP tests, verified CI race/multi-arch build results,
> additional application issues, and actual base-image OS CVE findings.

## Scope and limitations

Review of application authorization, credentials, SSH/HTTP integrations, frontend
type/build failures, background jobs, and Go/npm dependency advisories. This is
not a penetration-test certification or a claim that all code is vulnerability-free.
Findings below distinguish source-level fixes from deployment validation.

## Verified issues addressed

- **Privilege escalation:** privileged usernames no longer bypass RBAC; removed
  automatic promotion of accounts named admin/administrator/root. Administrator
  privilege is role-based. Initial-admin creation is transactional.
- **Resource authorization:** Docker engine operations and linked SSH credentials
  require access to the selected engine/host. Engine sharing does not reveal
  private containers. Public-container reads remain supported. Container edits
  cannot spoof ownership. Integration services check the actor's profile access;
  invalid OpenSearch/Data Prepper IDs no longer select a different profile.
- **Write boundaries:** OpenSearch arbitrary mutating queries require feature and
  profile manage access. API endpoints must be relative paths. Selected saved
  integration profiles are checked for configuration/pipeline writes. SSH and
  Grafana tests cannot reuse saved credentials against a changed destination.
- **Terminal access:** WebSocket sessions require feature/resource manage access,
  and initial authentication frames are size/time bounded.
- **Sessions:** password changes revoke sessions; removed ten-year sliding
  extensions and imposed a 30-day absolute maximum.
- **Credentials:** malformed AES-GCM nonce/tag inputs return errors instead of
  panicking; encrypted database configuration writes fail closed with restrictive
  file modes. Encryption initialization precedes decryption. Production rejects
  short/default/example keys. PostgreSQL connection URLs and database identifiers
  are escaped. New Grafana tokens are encrypted at rest and API responses mask
  them. Legacy plaintext tokens remain readable for upgrade compatibility.
- **Transport:** persisted SSH TOFU rejects changed keys; strict mode supports
  pre-provisioned known_hosts. Legacy algorithms require explicit opt-in. SSH
  handshake deadlines are enforced. Generic reporting/status-page HTTPS verifies
  certificates by default. Private status pages authenticate before live probes.
- **HTTP:** security headers, request size limits, and query-secret log redaction.
- **Queue continuity:** stopped pools reject new jobs without sending on a closed
  channel; cancellation interrupts retry backoff, handler panics become failed
  jobs, pending jobs can be cancelled, readers receive job snapshots, and active
  jobs are not evicted to make room for history.
- **Frontend/runtime:** production builds now enforce TypeScript checks. Fixed
  undefined template/functions/fields, ECharts registration, IPAM, topology, SFTP
  mutable transfer state, nullable values, and unused code/imports. Removed
  fabricated report telemetry on failed queries; unavailable data stays unavailable.
  Adapted chunk configuration to Vite 8.

## Dependencies and CVEs

Baseline Go scan reported **100 unique advisory IDs**, including **55 with
called-symbol traces** on the scanned Windows target. Package/module findings
are not proof of exploitation; call traces also require deployment-specific
assessment. After upgrading to Go **1.27.4** and updated dependencies,
`govulncheck -json ./...` completed successfully with **0 finding records** on
both Windows and the cross-targeted Linux scan (`GOOS=linux CGO_ENABLED=0`).
The scanner was rebuilt with the project Go toolchain, not an older Go binary.

Continuity-relevant examples from the baseline:

| Advisory | Potential impact | Remediation |
| --- | --- | --- |
| GO-2026-5017 / CVE-2026-39830 | SSH deadlock | x/crypto 0.59.0 |
| GO-2026-5018 / CVE-2026-39829 | SSH key-parsing DoS | x/crypto 0.59.0 |
| GO-2026-6354, GO-2026-6355 | SSH channel deadlocks | x/crypto 0.59.0 |
| GO-2026-5004 / CVE-2026-41889 | pgx placeholder confusion/SQL injection under affected query conditions | pgx 5.13.0 |
| GO-2026-5764 / GHSA-xmrv-pmrh-hhx2 | AWS EventStream decoder panic | updated AWS SDK graph |
| GO-2026-6603 / CVE-2026-78659 | HTTP/2 memory exhaustion | Go 1.27.4 / x/net 0.63.0 |
| GO-2026-6859 / CVE-2026-97054 | HTTP/1 server crash | Go 1.27.4 |

Baseline npm audit: **6 vulnerable packages (1 high, 5 moderate)**. Updated Vite
8.3.8, ECharts 6.1.0, Axios 1.21.1 and build tooling; committed lockfile captures
the resolved graph. Post-upgrade `npm audit --json`: **0 vulnerabilities**.
Baseline examples include ECharts GHSA-fgmj-fm8m-jvvx, esbuild
GHSA-67mh-4wv8-2f99, Vite GHSA-4w7w-66w2-5vf9, and minimatch
GHSA-g3hv-75wg-m249. Dev-server findings do not imply the static Nginx production
server exposes the same attack surface.

Dockerfiles now target Go 1.27.4, Node 24, Alpine 3.24.4, Nginx 1.30.7 and locked
installs; obsolete duplicate PostgreSQL 16 client installation was removed.
**These images have not been built/scanned locally.** Updating a base tag is not
evidence that every OS package is free of CVEs.

## Validation

- `npm run build`: type-check and production bundle passed (baseline: 245 TS diagnostics).
- `go test ./...` and `go vet ./...`: passed after security/queue tests were added.
- Linux cross-build (`GOOS=linux CGO_ENABLED=0 go build ./...`): passed.
- Queue tests repeated 50 times: passed (not a substitute for race detection).
- Regression tests cover role-vs-username, malformed ciphertext, SSH enrollment/
  changed keys, OpenSearch query policy, log redaction, queue panic survival,
  cancellation, snapshots, queue saturation and shutdown.
- Chrome/Playwright mocked API regressions: 10 viewport/theme combinations,
  responsive drawer/focus/resize, RBAC navigation, scoped Docker counts, empty
  state and stale-response protection passed; no runtime errors in those checks.
- Local race detection unavailable: no CGO compiler. CI now runs `go test -race
  ./...` on Linux, plus vet, build, Go vulnerability scan, npm audit/type/build.
- Live PostgreSQL migrations/concurrency, SSH servers, S3, Grafana, OpenSearch,
  Prometheus, Data Prepper, OTel and Docker deployment were **not** integration-tested.

## Upgrade checklist

1. Back up the database, encrypted configuration, and the **existing encryption
   key**. Do not replace an existing valid key: credentials would become unreadable.
   Short/example keys now prevent production startup; changing such a key requires
   planned credential migration/re-entry, not an in-place random replacement.
2. Use Go 1.27.4 / Node 24, `go mod download`, `npm ci`. Rebuild both images and
   validate OS packages and all published architectures in CI/staging.
3. Persist `DATA_DIR` and known_hosts. For strict SSH, pre-populate trusted keys,
   set `SSH_HOST_KEY_POLICY=strict` and optionally `SSH_KNOWN_HOSTS`. TOFU cannot
   protect the first connection from an active MITM. Investigate changed keys;
   do not delete entries merely to silence errors.
4. Old SSH hosts may require `SSH_ALLOW_LEGACY_ALGORITHMS=true` temporarily.
   Prefer upgrading servers. Install private CAs for HTTPS. Global
   `ALLOW_INSECURE_INTEGRATION_TLS=true` is a temporary unsafe escape hatch for
   generic reporting/status-page clients, not a recommended setting.
5. Review explicitly assigned administrator roles; removing username promotion
   does not undo privileged roles already stored in the database. Review shared
   profile permissions and expect previously unauthorized operations to return 403.
6. Re-enter/save existing Grafana tokens to encrypt historical plaintext values;
   unchanged legacy tokens and old backups are not automatically rewritten.
7. Test database backups/restores, private/public status pages, session expiry,
   SSH/SFTP, OpenSearch read/write RBAC, Docker ownership/sharing and all integrations.

## Residual risk / follow-up

- HCP is a privileged infrastructure control plane. Docker manage access (local
  sockets, bind mounts, deployment settings) and SSH manage/terminal access can
  be host-root equivalent. Grant only to trusted operators; isolate engines and
  use least-privilege service accounts. This audit does not sandbox Docker admins.
- Internal addresses are intentional monitoring targets. A blanket private-IP
  SSRF ban would break the product; enforce network egress allowlists externally
  and review profile creation rights, redirects, DNS rebinding and target policy.
- OpenSearch per-profile `VerifySSL=false` remains an explicit insecure option.
  RDP inventory certificate discovery inspects untrusted certificate metadata
  without sending credentials; it does not authenticate the remote host.
- API query-token compatibility still exists; application logs redact secrets,
  but reverse-proxy/access logs and browser history need separate review. Prefer
  header/cookie authentication, secure ingress and short-lived credentials.
- Continue reviewing every integration endpoint and credential response, not
  only the paths fixed here; broad live cross-user matrix tests remain necessary.
- A credential was found embedded in the local Git remote configuration. Do not
  copy it into reports/logs; replace with a credential manager and rotate it.

Advisory results are time- and target-specific. Re-run scans on Linux production
architectures and scan built images before release; zero findings is not a security guarantee.

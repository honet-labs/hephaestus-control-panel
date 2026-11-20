# Real vendor integration fixtures

These are disposable test services, not production deployment examples. Ports
are bound to loopback, credentials are synthetic, image digests are pinned, and
CI removes containers plus their anonymous volumes after every job. Do not add
customer credentials/data to these files. Do not deploy this Compose stack publicly.

The `Isolated Vendor Integration Audit` workflow starts one vendor per job and
executes `TestVendorLive` with Linux race detection:

| Vendor | Actual check |
| --- | --- |
| OpenSearch 3.10.0 | HCP connection/auth/TLS policy, index/document and HCP report query |
| Prometheus 3.16.0 | HCP live range query and returned metric values |
| Grafana 13.3.1 + Prometheus | Service-account token, datasource, HCP query through Grafana, invalid-token denial |
| Data Prepper 2.16.0 | Same pipeline accepted by HCP YAML validation and running Data Prepper HTTP source |
| OTel Collector Contrib 0.165.0 | Collector startup, real OTLP metrics ingestion and Prometheus export |
| Vaultwarden 1.37.5 | Account bootstrap, HCP login, password-bound cache, encrypted cipher create/update/sync/decrypt/delete |
| RustFS 1.0.6 (S3-compatible) | Signed HCP upload, object download/content verification, invalid-secret denial |

No vendor test is marked as passing merely because this workflow exists: use the
workflow run/artifact for the tested commit. Data Prepper/OTel process-management
over a customer's SSH/systemd installation is not reproduced by these containers.
The separate SSH/SFTP protocol suite covers the transport; historical customer
schemas, production routing/certificates, and deployment still require access and
authorization. These images are integration fixtures, not HCP production images
and not a guarantee of zero upstream vendor CVEs.

For local Docker use, start the selected profile, set `HCP_AUDIT_VENDOR` to its
name, and run `go test ./internal/services -run '^TestVendorLive$' -v -count=1`.
The Vaultwarden test additionally needs a disposable loopback PostgreSQL account
with CREATE DATABASE and the `HCP_AUDIT_POSTGRES_*` variables used by CI.

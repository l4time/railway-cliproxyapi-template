# Architecture and Security Boundary

## Runtime components

| Component | Pin / behavior | Boundary |
|---|---|---|
| CLIProxyAPI | `v7.2.141`, immutable index digest in `release-state.json` | Loopback `127.0.0.1:8317`; never directly public |
| Management Center | `v1.22.6`, SHA-256 `e2643e08…2b721f4` | Bundled local static asset; upstream auto-download disabled |
| Health proxy | Package-owned, rootless PID 1 after initialization | Public `:8080`; `/healthz` plus reverse proxy |
| Entrypoint + config reconciler | Package-owned, exact proved sources | Narrow root volume/config initialization and atomic key reconciliation, then UID/GID `10001` |
| Volume | Railway `/data` | Provider auth, rootless home, and protected persistent config/state; one replica only |

The public health endpoint tests only loopback TCP readiness and returns `ok`.
It does not return models, configuration, provider state, account identifiers,
or secrets. Every proxied application route retains upstream authentication.

## Secret lifecycle

The entrypoint reads the two generated keys only long enough to validate them.
A static helper opens `/data/state` without following symlinks, requires exact
UID/GID `10001` and mode `0700`, validates any existing regular single-link
config at mode `0600`, parses an exact unambiguous grammar, retains only
`debug`, `request-retry`, `max-retry-credentials`, and `max-retry-interval`,
reconciles the two Railway keys, reasserts every wrapper-owned security field,
and atomically replaces `/data/state/config.yaml`. Anchors, aliases, tags,
inline collections, directives, multi-document markers, malformed prefixes,
duplicates, unknown fields, and credential order/cardinality drift fail closed.
It unsets source variables before `exec`. Tests inspect image history, process
arguments, the child environment, logs, and health output for absence of
generated values.

Keys still exist in Railway's encrypted variable plane and in the persistent
config. Operators with sufficient Railway/service or volume access remain
trusted. Provider state and `/data/state/config.yaml` are durable credential
material and must be encrypted in backups.

## Trust model

Supported:

- One trusted operator or a tightly controlled trusted team.
- Clients that keep the proxy key secret.
- Remote administration over Railway HTTPS with the distinct management key.
- Provider accounts the operator is authorized to connect.

Not supported:

- Public unauthenticated relay or anonymous multi-user service.
- Per-user quotas, RBAC, tenant isolation, or abuse prevention supplied by this
  package.
- Shared/resold subscriptions or evasion of provider authentication, policy,
  billing, rate limits, geographic restrictions, or safety controls.
- Multiple replicas sharing one filesystem volume.
- A runtime process with a Railway or GitHub token.

## Hard stops

Pause publication or promotion on auth bypass, equal/default keys, public
upstream port, missing volume, unexpected panel download, checksum drift,
secret output, root runtime, extra service, multiple replicas, Serverless,
unproved state migration, provider policy conflict, or failed cleanup.

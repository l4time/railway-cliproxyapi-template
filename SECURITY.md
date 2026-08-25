# Security Policy

## Supported package scope

Report privately when a defect concerns:

- Missing, default, equal, exposed, or incorrectly routed proxy/management
  keys.
- Public access to protected proxy or management routes.
- Secret leakage through source, image history, process arguments/environment,
  logs, health, persisted config, backups, tests, issues, assets, or release
  automation.
- A Management Center checksum/pin bypass or remote download fallback.
- Incorrect loopback/public port, root-drop, `/data`, health, replica,
  Serverless, restart, update, or rollback wiring.
- Runtime-updater or release-controller acceptance of a draft/prerelease,
  mutable/malformed source, under-soak release, checksum/tag reuse, unsafe
  archive, failed private probe, public updater surface, or failed rollback.
- Misleading provider authorization, subscription, backup, recovery, or trust
  guidance controlled by this package.

CLIProxyAPI or Management Center vulnerabilities belong to their upstream
security processes. Provider and Railway platform vulnerabilities belong to
those vendors. Do not publish suspected credentials or exploit details.

## Private reporting

Use GitHub **Security → Advisories → Report a vulnerability**:

`https://github.com/l4time/railway-cliproxyapi-template/security/advisories/new`

If private advisories are unavailable, use the repository owner's private
GitHub profile contact to request a secure channel. Include package commit,
public runtime/panel pin, impact, sanitized reproduction, and whether any
credential was exposed.

## Never include

- Proxy or management keys.
- Provider OAuth/device codes, cookies, refresh/access tokens, account files,
  subscription identifiers, or `/data` archives.
- Authorization headers, prompts, model responses, private domains, Railway
  tokens/IDs, complete environment dumps, or unredacted logs.

If provider authorization may be exposed, revoke it at the provider first.
Rotate the affected template key separately. Removing an issue or rotating only
the proxy key does not invalidate provider-side credentials.

## Accepted defaults

- Two distinct generated keys with no default.
- Railway HTTPS to rootless proxy `:8080`; CLIProxyAPI loopback only.
- Pinned local Management Center; its updater disabled.
- App/provider state only on one `/data` volume and one replica.
- Protected management configuration at `/data/state/config.yaml`, owned by
  UID/GID `10001`, mode `0600`, with fail-closed path checks and atomic
  credential reconciliation at boot.
- Canonical boot-time reassertion of loopback `127.0.0.1:8317`, TLS off,
  authenticated remote management, pinned-panel controls, `/data/auth`,
  file logging/statistics off, and WebSocket auth on. Persisted YAML cannot
  override these wrapper-owned fields.
- Serverless off and `ON_FAILURE` with a finite maximum of 10 retries.
- Private runtime stable-release updater without Railway/GitHub/app/provider
  credentials; protected ledger and binaries under `/data/update`.
- External release controller remains build-time qualification/canary defense.

Changing these defaults is outside the proved template contract.

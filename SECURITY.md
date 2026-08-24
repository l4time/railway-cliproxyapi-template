# Security Policy

## Supported package scope

Report privately when a defect concerns:

- Missing, default, equal, exposed, or incorrectly routed proxy/management
  keys.
- Public access to protected proxy or management routes.
- Secret leakage through source, image history, process arguments/environment,
  logs, health, tests, issues, assets, or release automation.
- A Management Center checksum/pin bypass or remote download fallback.
- Incorrect loopback/public port, root-drop, `/data`, health, replica,
  Serverless, restart, update, or rollback wiring.
- Release-controller acceptance of a draft/prerelease, mutable/malformed
  source, under-soak release, excess promotion, or failed smoke.
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
- Serverless off and one ON_FAILURE retry.
- External release controller without Railway/app/provider credentials.

Changing these defaults is outside the proved template contract.

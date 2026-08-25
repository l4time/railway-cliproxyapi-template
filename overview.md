# Deploy and Host CLIProxyAPI — Daily Auto-Update

## About Hosting CLIProxyAPI

CLIProxyAPI is an open-source compatibility gateway for connecting authorized
provider accounts and exposing compatible model APIs. This template packages
it as a secure, persistent, release-tracked Railway service.

## Why Deploy CLIProxyAPI — Daily Auto-Update?

The package adds a rootless steady state, loopback-only upstream, pinned
management asset, explicit authentication separation, an immutable embedded
fallback, and a verified runtime stable-release updater with private candidate
probing and binary-only rollback. Railway supplies HTTPS, secret generation,
image builds, health checks, restart policy, metrics, and persistent storage.
Each instance checks immediately on boot when overdue, then every 6 hours plus
at most 30 minutes of deterministic jitter, with retry clamping that preserves
an attempt within every rolling 24 hours while continuously running. A newer
official stable release must soak for 6 hours and pass checksum, archive,
private-probe, authenticated live-state, cutover, and probation gates.

## Common Use Cases

- A private API endpoint for Codex and other compatible clients.
- Remote, browser-based provider authorization through a pinned management
  panel.
- One trusted operator managing several authorized provider connections.
- A gateway that follows official stable releases after supply-chain and
  private runtime checks, while retaining a verified fallback and prior binary.

## Dependencies for CLIProxyAPI Hosting

Provider accounts, subscriptions, credentials, and usage charges are not
included. Operators must have authorization and follow provider terms.

### Deployment Dependencies

The template creates one public service and one `/data` volume. Railway
generates different proxy and administration keys. The proxy key authenticates
clients; the administration key protects Management Center and management API
operations.

The default is one always-on replica with Serverless disabled. No database,
Redis, Bucket, worker, shared credential, provider account, Railway token, or
GitHub token is bundled. The private updater uses anonymous official-release
reads and persists its bounded ledger under `/data/update`. Backups contain
sensitive authorization state and must be encrypted.

Automatic rollback replaces only the executable and quarantines the exact
deterministically bad release identity. It never rewinds live provider,
configuration, or authorization state; an incompatible irreversible upstream
migration can still require an operator-restored stopped backup.

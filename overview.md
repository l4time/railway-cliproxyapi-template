# Deploy CLIProxyAPI — Secure Release-Tracked

## About

CLIProxyAPI is an open-source compatibility gateway for connecting authorized
provider accounts and exposing compatible model APIs. This template packages
it as a secure, persistent, release-tracked Railway service.

## Common use cases

- A private API endpoint for Codex and other compatible clients.
- Remote, browser-based provider authorization through a pinned management
  panel.
- One trusted operator managing several authorized provider connections.
- A digest-pinned gateway with tested release promotion and rollback.

## Deployment dependencies

The template creates one public service and one `/data` volume. Railway
generates different proxy and administration keys. The proxy key authenticates
clients; the administration key protects Management Center and management API
operations.

Provider accounts, subscriptions, credentials, and usage charges are not
included. Operators must have authorization and follow provider terms.

## Why Railway?

Railway supplies HTTPS, secret generation, image builds, health checks, restart
policy, metrics, and persistent storage. The package adds a rootless steady
state, loopback-only upstream, pinned management asset, explicit auth
separation, and an external stable-release controller.

The default is one always-on replica with Serverless disabled. No database,
Redis, Bucket, worker, runtime updater, shared credential, or provider account
is bundled. Backups contain sensitive authorization state and must be encrypted.

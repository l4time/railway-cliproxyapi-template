# Marketplace Overview

## Listing identity

- Name: `CLIProxyAPI — Daily Auto-Update`
- Proposed code: `cliproxyapi-secure-release-tracked`
- Suggested category: `AI`
- Repository: `https://github.com/l4time/railway-cliproxyapi-template`
- Proposed deploy URL:
  `https://railway.com/deploy/cliproxyapi-secure-release-tracked`

## Short description

Run CLIProxyAPI as a private gateway with separate generated API/admin keys,
remote provider authorization, persistent state, an immutable embedded
fallback, and verified stable-release update/rollback.

## Overview copy

CLIProxyAPI connects authorized provider accounts to compatible model APIs.
This template deploys one rootless Railway service with one `/data` volume,
separate generated proxy and management keys, and a checksum-pinned browser
management panel.

The upstream process listens only on loopback behind a small health/reverse
proxy. Provider authorization persists under `/data/auth`; remote operators use
the management key, while Codex and other compatible clients use the distinct
proxy key.

The private runtime supervisor checks official stable semantic upstream
releases every 6 hours plus bounded jitter, waits 6 hours, verifies the exact
architecture archive against upstream checksums, probes it on loopback, and
retains prior and embedded binaries for binary-only rollback. Overdue boots
check immediately, and a continuously running instance attempts within every
rolling 24 hours. It has no public updater route, Railway token, GitHub token,
or provider credential. The external controller remains a build-time canary
for embedded fallbacks.

Provider accounts, subscriptions, credentials, shared access, and usage are not
included. Operators must be authorized and follow provider terms. The package
does not guarantee provider compatibility, permission, model behavior, cost,
privacy, uptime, GitHub availability, or persisted-state downgrade
compatibility. Automatic rollback never rewinds live user state; irreversible
upstream migrations require explicit stopped-backup recovery.

## Services and inputs

- One `cliproxyapi` service on port `8080`.
- One volume at `/data`.
- Two hidden generated secret variables; no user prompts.
- No database, Redis, Bucket, worker, scheduler, or provider.

## First run

Wait for `/healthz`, save the two generated keys separately, open
`/management.html` with the management key, connect an authorized provider,
then configure clients with the public base URL and proxy key.

## Asset plan

The published listing uses the accepted no-custom-icon/no-screenshot exception
to avoid copied upstream/provider marks and credential-bearing UI. Railway may
use a neutral platform default. Any future custom asset must be original and
provenance-recorded before publication.

## Draft review

Verify one service/domain/volume, exact pins, `/healthz`, `8080`, one replica,
Serverless off, ON_FAILURE maximum 10 retries, both distinct secret generators
and descriptions, no prompts/defaults, no hidden service, correct overview
copy, and working public support/security links.

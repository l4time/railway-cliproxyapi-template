# Marketplace Overview

## Listing identity

- Name: `CLIProxyAPI — Secure Release-Tracked`
- Proposed code: `cliproxyapi-secure-release-tracked`
- Suggested category: `AI`
- Repository: `https://github.com/l4time/railway-cliproxyapi-template`
- Proposed deploy URL:
  `https://railway.com/deploy/cliproxyapi-secure-release-tracked`

## Short description

Run CLIProxyAPI as a private, digest-pinned gateway with separate generated API
and admin keys, remote provider authorization, persistent state, and tested
stable-release rollback.

## Overview copy

CLIProxyAPI connects authorized provider accounts to compatible model APIs.
This template deploys one rootless Railway service with one `/data` volume,
separate generated proxy and management keys, and a checksum-pinned browser
management panel.

The upstream process listens only on loopback behind a small health/reverse
proxy. Provider authorization persists under `/data/auth`; remote operators use
the management key, while Codex and other compatible clients use the distinct
proxy key.

An external GitHub controller watches stable semantic upstream releases, waits
12 hours, resolves an immutable image digest, runs complete auth/UI/state and
update/rollback smoke, limits promotions, and retains the prior digest. The
running app contains no updater or Railway token.

Provider accounts, subscriptions, credentials, shared access, and usage are not
included. Operators must be authorized and follow provider terms. The package
does not guarantee provider compatibility, permission, model behavior, cost,
privacy, uptime, or automatic consumer updates.

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

The initial listing requests an explicit no-custom-icon/no-screenshot exception
to avoid copied upstream/provider marks and credential-bearing UI. Railway may
use a neutral platform default if required. QA and the publish packet must
accept the exception or provide an original, provenance-recorded asset before
publication.

## Draft review

Verify one service/domain/volume, exact pins, `/healthz`, `8080`, one replica,
Serverless off, ON_FAILURE one retry, both distinct secret generators and
descriptions, no prompts/defaults, no hidden service, correct overview copy,
and working public support/security links.

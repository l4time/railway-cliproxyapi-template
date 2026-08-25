# Operations Guide

## Routine checks

- Expected topology: one `cliproxyapi` service, one HTTPS domain, one `/data`
  volume, one replica, Serverless off.
- Health: `GET /healthz` must return status `200` and body `ok`.
- Missing/wrong credentials: `/v1/models` and `/v0/management/config` must
  return `401`.
- Correct proxy key: `/v1/models` may return the connected model list.
- Correct management key: `/v0/management/config` must return `200`.
- Cross-key checks: each key must fail against the other surface.
- Panel: `/management.html` body SHA-256 must remain
  `e2643e0875e0024e5ff9ddf4569e4c58611ab0456aeb6fa6065ed3e6c2b721f4`.

Use a synthetic or low-risk provider request for routine verification. Never
print response content that may contain prompts, account identifiers, quotas,
or provider metadata.

## Logs and metrics

Fetch only a bounded time range. Remove both template keys, Authorization
headers, OAuth/device codes, cookies, provider tokens, account identifiers,
model prompts/responses, Railway IDs, and private domains before sharing.

Normal startup should identify the CLIProxyAPI version without printing
credentials. The updater may log only sanitized attempt/no-op/promotion/
rollback outcomes. Any management-panel download/fallback message or public
updater route is a hard stop: the image must serve the bundled asset instead.

Investigate sustained resource growth against the Railway proof baseline
rather than promising a fixed cost. The corrected proof peaked near 30 MB and
very low CPU with no provider workload; real traffic and provider behavior
change usage.

## Backup

1. Stop the service or quiesce writes.
2. Export `/data/auth`, `/data/home`, `/data/state`, and `/data/update` with
   ownership, modes, hidden files, and checksums.
3. Exclude and do not modify `/data/lost+found`; it is ext4 metadata.
4. Encrypt the archive and restrict it like provider credentials.
5. Record image tag/digest, panel version/checksum, and backup time.
6. Restart and recheck health and both auth surfaces.

## Restore

1. Use a disposable or approved recovery service with one fresh `/data`.
2. Keep it stopped during restore.
3. Restore the four app-owned trees without weakening modes or moving secrets
   outside `/data/auth` and `/data/state/config.yaml`.
4. Start the same accepted image first.
5. Verify health, key separation, panel checksum, provider state, and a
   non-sensitive request.
6. Revoke and reauthorize any provider whose backup custody is uncertain.
7. If config custody is uncertain, rotate both Railway template keys and prove
   that each old key returns `401`.

This package does not claim an online copy is consistent or that Railway
volume replacement is atomic.

## Key rotation

Rotate one layer at a time:

1. Save a stopped encrypted backup.
2. Change either proxy or management variable.
3. Redeploy. Boot atomically reconciles that variable into the persistent
   config while preserving the explicit non-secret allowlist: `debug`,
   `request-retry`, `max-retry-credentials`, and `max-retry-interval`.
4. Verify the new value succeeds, the old value fails, and a non-secret
   management setting still survives restart.
5. Update trusted clients/secret manager.
6. Rotate provider authorization separately at the provider.

## Update

The rootless supervisor checks the official upstream GitHub stable-release feed
every 6 hours plus up to 30 minutes of deterministic jitter. An absent,
overdue, or implausibly future schedule triggers an immediate boot check;
transient retries remain within a rolling 24-hour attempt ceiling.

Only a forward exact semantic tag that has soaked 6 hours can proceed. The
supervisor verifies official architecture-specific archive/checksum assets,
strictly extracts one executable, records checksum immutability, privately
probes version/readiness/proxy-management separation/UI/exit, then performs a
graceful binary cutover with bounded probation. No token or public updater
endpoint exists. Management Center remains independently pinned.

Inspect `/data/update/ledger.json` only through trusted volume/service access.
It contains versions, hashes, cadence, observed ETag, phase, sanitized failure
classification, and exact `tag@checksum` quarantine
reasons—not application/provider credentials. Before high-value updates, keep
a stopped encrypted backup because a private disposable probe cannot prove
every provider-specific state migration.

## Rollback

Failed candidate start, authenticated live semantic validation, crash, or
bounded probation triggers an
automatic binary-only rollback to the retained verified prior executable. A
restart during staging discards the candidate; a restart during cutover or
probation recovers the prior binary from the phase journal. User state is
neither rewound nor replaced.

If the prior executable cannot safely use current persisted state, stop the
service and explicitly restore the encrypted pre-update archive with the last
proved image, then revoke/re-authorize provider access if necessary. The
external repository workflow remains available for embedded-image rollback,
but it is not the consumer runtime rollback mechanism.

## Failure modes

| Symptom | Safe response |
|---|---|
| `secure initialization failed` | Check two distinct generated variables plus exact `/data`/app-directory/config ownership and modes; reject symlinks or malformed config rather than weakening validation |
| A non-allowlisted Management API setting resets | Expected wrapper policy; only debug and the three documented bounded retry fields persist |
| Health `503` | Inspect bounded startup logs and loopback child status; do not expose port `8317` |
| Proxy `401` | Use the proxy key as Bearer; do not substitute the management key |
| Management `401` | Use the management key; confirm cross-key separation |
| Panel changes or fetches remotely | Stop; verify image checksum/pin and `MANAGEMENT_STATIC_PATH` |
| Provider authorization fails | Check upstream/provider status and terms; never request another user's credentials |
| State missing after restart | Stop writes; confirm exact `/data` attachment before reauthorizing |
| Update remains no-op | Confirm current tag, stable exact upstream tag, 6-hour soak, GitHub availability, and persisted next-check/ETag |
| Tag is quarantined | Treat as fail-closed; inspect upstream release/checksum history and package QA before clearing state |
| Candidate probe fails | Current binary keeps serving; inspect sanitized version/readiness/auth/UI evidence without exposing probe credentials |
| Runtime rollback occurs | Stop promotion, inspect phase/quarantine/upstream migration notes, and validate provider state before any retry |

## Incident priority

- S0: key/provider-token disclosure, auth bypass, unsafe panel supply chain,
  public relay, or data-integrity risk.
- S1: clean deploy, health, volume, restart, update, or rollback failure.
- S2: documentation, editor serialization, setup, or support-routing defect.
- S3: optional enhancement or upstream/provider limitation.

S0/S1 pauses promotion and publication. Revoke exposed provider credentials at
their source; changing only a proxy key is insufficient.

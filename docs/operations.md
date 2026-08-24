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
credentials. Any management-panel download/fallback message is a hard stop:
the image must serve the bundled asset instead.

Investigate sustained resource growth against the Railway proof baseline
rather than promising a fixed cost. The corrected proof peaked near 30 MB and
very low CPU with no provider workload; real traffic and provider behavior
change usage.

## Backup

1. Stop the service or quiesce writes.
2. Export `/data/auth`, `/data/home`, and `/data/state` with ownership, modes,
   hidden files, and checksums.
3. Exclude and do not modify `/data/lost+found`; it is ext4 metadata.
4. Encrypt the archive and restrict it like provider credentials.
5. Record image tag/digest, panel version/checksum, and backup time.
6. Restart and recheck health and both auth surfaces.

## Restore

1. Use a disposable or approved recovery service with one fresh `/data`.
2. Keep it stopped during restore.
3. Restore the three app-owned trees without weakening modes or moving secrets
   outside `/data/auth`.
4. Start the same accepted image first.
5. Verify health, key separation, panel checksum, provider state, and a
   non-sensitive request.
6. Revoke and reauthorize any provider whose backup custody is uncertain.

This package does not claim an online copy is consistent or that Railway
volume replacement is atomic.

## Key rotation

Rotate one layer at a time:

1. Save a stopped encrypted backup.
2. Change either proxy or management variable.
3. Redeploy and verify the new value succeeds and the old value fails.
4. Update trusted clients/secret manager.
5. Rotate provider authorization separately at the provider.

## Update

The scheduled repository controller evaluates only stable semantic releases,
requires a 12-hour soak, resolves the exact Docker manifest digest, limits
promotion frequency, and runs the complete container smoke before committing.
The Management Center does not move with CLIProxyAPI automatically.

Before accepting an update, review upstream security/migration/provider notes
and save a stopped encrypted backup. After deployment, recheck all routine
checks, provider authorization, restart persistence, logs, and resources.

## Rollback

Run the repository workflow manually with operation `rollback`. It swaps the
current and retained prior records, changes both Dockerfile pins, and runs the
complete target-only smoke. It never builds or boots the outgoing current
image. The retained target must pass every auth/UI/state/restart/secret/resource
gate, and the workflow commits only after success. It never changes provider
state.

If the prior image cannot safely use current state, do not force a binary-only
rollback. Restore the pre-update archive with the last proved image, then
revoke/re-authorize provider access if necessary.

## Failure modes

| Symptom | Safe response |
|---|---|
| `secure initialization failed` | Check two distinct generated variables and `/data` ownership; do not weaken validation |
| Health `503` | Inspect bounded startup logs and loopback child status; do not expose port `8317` |
| Proxy `401` | Use the proxy key as Bearer; do not substitute the management key |
| Management `401` | Use the management key; confirm cross-key separation |
| Panel changes or fetches remotely | Stop; verify image checksum/pin and `MANAGEMENT_STATIC_PATH` |
| Provider authorization fails | Check upstream/provider status and terms; never request another user's credentials |
| State missing after restart | Stop writes; confirm exact `/data` attachment before reauthorizing |
| Release workflow defers | Read decision (`soak`, daily limit, duplicate); do not bypass the gate |
| Release smoke fails | Leave current pin unchanged; inspect sanitized test evidence |

## Incident priority

- S0: key/provider-token disclosure, auth bypass, unsafe panel supply chain,
  public relay, or data-integrity risk.
- S1: clean deploy, health, volume, restart, update, or rollback failure.
- S2: documentation, editor serialization, setup, or support-routing defect.
- S3: optional enhancement or upstream/provider limitation.

S0/S1 pauses promotion and publication. Revoke exposed provider credentials at
their source; changing only a proxy key is insufficient.

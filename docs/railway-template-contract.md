# Railway Template Contract

Template candidate: `cliproxyapi-secure-release-tracked`  
Public product name: `CLIProxyAPI — Secure Release-Tracked`  
Package date: 2026-08-25  
Kit version: `2026-07-04-v1`

## Topology

| Resource | Exact contract |
|---|---|
| Application services | One, named `cliproxyapi` |
| Source | Repository root Dockerfile |
| Public domain | One HTTPS domain targeting `8080` |
| Volume | One Railway volume mounted at `/data` |
| Health | `GET /healthz`, timeout 60 seconds |
| Replicas | One |
| Restart | `ON_FAILURE`, maximum 10 retries |
| Serverless | Disabled |
| Other resources | No database, Redis, Bucket, worker, scheduler, TCP proxy, or second service |

Do not expose CLIProxyAPI's loopback port `8317`, attach `/data` to multiple
replicas, or add an editor start-command override.

## Required template variables

| Name | Exact expression | Description | Prompt/default |
|---|---|---|---|
| `CLIPROXY_PROXY_KEY` | `${{secret(64)}}` | Generated Bearer key for API clients. Save privately; not for administration. | Generated/hidden; no prompt or default |
| `CLIPROXY_MANAGEMENT_KEY` | `${{secret(64)}}` | Generated administration key for Management Center and management API. Save separately; not for normal clients. | Generated/hidden; no prompt or default |

They must be separate generator expressions, not one shared reference. A clean
draft and public consumer smoke must prove distinct values, missing/wrong/key
separation behavior, and secret absence from public metadata/logs.

`PORT` is supplied by Railway; the image defaults to and exposes `8080`.
`MANAGEMENT_STATIC_PATH` and `HOME` are image/entrypoint-owned and must not be
template variables.

## Runtime pins

| Component | Accepted pin |
|---|---|
| CLIProxyAPI | `v7.2.141`; `eceasy/cli-proxy-api@sha256:7f598ce64478a8a5f90ed76875e0e9b0e7d77b80e17184b13df18c3d5bdb3def` |
| Management Center | `v1.22.6`; `management.html` SHA-256 `e2643e0875e0024e5ff9ddf4569e4c58611ab0456aeb6fa6065ed3e6c2b721f4` |
| Steady-state identity | UID/GID `10001`; no new privileges |

The release ledger may advance the CLIProxyAPI tag/digest only through the
tested external controller. The panel pin changes separately after review and
full regression.

## HTTP behavior

| Path | Expected public behavior |
|---|---|
| `/healthz` | Unauthenticated `200`, body `ok` only |
| `/v1/*` | Missing/wrong key `401`; correct proxy key accepted |
| `/v0/management/*` | Missing/wrong/cross key `401`; correct management key accepted |
| `/management.html` | Local bundled asset with accepted SHA-256 |

The proxy key must not authorize management. The management key must not be
used by normal clients. The management page may be fetched, but privileged API
operations remain management-key protected.

## Persistence

App-owned state is `/data/auth`, `/data/home`, and `/data/state`.
`/data/lost+found` may exist as root-owned ext4 metadata and is excluded from
app ownership and backups without being deleted or permission-weakened.

One stopped encrypted backup must precede updates. A live copy, shared-volume
replica, binary-only state downgrade, or atomic replacement-volume operation is
not promised.

## Release-controller contract

- Official `router-for-me/CLIProxyAPI` GitHub releases only.
- Stable exact `vMAJOR.MINOR.PATCH`; no draft or prerelease.
- At least 12 hours since `published_at`.
- Candidate numeric semantic version strictly greater than current; lower
  candidates fail closed and equal current tag/digest is a no-op.
- Matching `eceasy/cli-proxy-api:<tag>` resolved to immutable SHA-256 manifest.
- Duplicate, malformed, known prior, and more-than-one-per-24-hour promotion
  rejected/deferred.
- Full auth, state, pinned-UI, transition, restart, child-failure, secret, and
  bounded-resource smoke before commit.
- Retain one prior tag/digest; emergency rollback smokes that target alone so a
  broken outgoing image is never required, while target failure blocks commit.
- Exact-format `SOURCE_SHA256SUMS` must verify before change and refresh to the
  new Dockerfile hash in the same release operation; checksum drift blocks.
- Repository GitHub token only; no Railway, template, or provider credential.

## Hard blockers

Any topology/variable drift, auth bypass, public `8317`, missing panel checksum,
remote panel fallback, secret leak, root steady state, state loss, unsafe
volume mode, untested release commit, multiple replicas, Serverless, broken
support/security link, or unsupported provider-access claim blocks publication
and promotion.

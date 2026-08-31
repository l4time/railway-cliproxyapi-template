# CLIProxyAPI — Daily Auto-Update

Deploy [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) as one
Railway service with an immutable embedded fallback, a verified in-service
stable-release updater, separate generated proxy and administration keys, a
persistent `/data` volume, and a checksum-pinned Management Center.

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/cliproxyapi-secure-release-tracked)

This is an independent community package. It is not an official CLIProxyAPI,
provider, or Railway product, and it does not include, share, sell, or bypass
provider credentials. You must connect accounts or API credentials that you
are authorized to use and follow every provider's subscription and automation
terms.

## What the template creates

- One public `cliproxyapi` service on port `8080`.
- One persistent Railway volume mounted at `/data`.
- One HTTPS domain with `GET /healthz` as the unauthenticated health check.
- Two different Railway-generated 64-hex secret values:
  `CLIPROXY_PROXY_KEY` for API clients and `CLIPROXY_MANAGEMENT_KEY` for
  administration.
- CLIProxyAPI `v7.2.141`, initially pinned to immutable image digest
  `sha256:7f598ce6…bdb3def`.
- A private runtime supervisor that checks the official GitHub stable release
  feed every 6 hours plus at most 30 minutes of deterministic jitter, verifies
  the architecture-specific archive and upstream checksums, probes a candidate
  privately, and performs binary-only cutover or rollback.
- Management Center `v1.22.6`, bundled at build time and verified with SHA-256
  `e2643e08…2b721f4`.

There is no database, Redis, Bucket, worker, scheduler, provider account,
provider token, shared credential, Railway token, or GitHub token in the
application. The updater uses anonymous HTTPS reads of official upstream
GitHub Releases. Serverless is off, there is one replica, and failed processes
get at most 10 automatic retries under Railway's finite `ON_FAILURE` policy.

## Architecture

```text
API client -- Bearer proxy key ------+
                                     |
Browser -- pinned /management.html --+--> Railway HTTPS :8080
                                            |
                            rootless supervisor/reverse proxy/updater
                                            |
                                  CLIProxyAPI 127.0.0.1:8317
                                            |
                    /data/auth  /data/home  /data/state  /data/update
```

This migration branch adds a loopback Responses compatibility process between
the public supervisor and CLIProxyAPI. It reconstructs stateless
`previous_response_id` tool chains for clients that send `store: false`, keeps
the bounded 24-hour cache under `/data/state`, and starts a new chain when a
compaction summary is detected. All non-Responses routes remain transparent
pass-throughs to CLIProxyAPI.

The root entrypoint performs only the narrow ownership/configuration bootstrap
needed for a fresh Railway volume. It validates both keys, initializes the
three app-owned directories, atomically initializes or reconciles the protected
mode-`0600` config at `/data/state/config.yaml`, removes source keys from the
child environment, and drops permanently to UID/GID `10001`. Existing
`debug`, `request-retry`, `max-retry-credentials`, and `max-retry-interval`
settings survive restart; the two Railway variables remain authoritative after
key rotation. Every wrapper-owned network, TLS, management, panel, auth-path,
logging, statistics, and WebSocket-auth baseline is reasserted on boot.
CLIProxyAPI is never bound directly to the public port. The updater has no
public endpoint; `/healthz` is the only unauthenticated wrapper route.

## Deploy

1. Open the Deploy on Railway link.
2. Confirm the draft creates exactly one service, one domain, and one volume at
   `/data`.
3. Confirm both key variables show generated secret expressions and have no
   literal or user-entered defaults.
4. Leave Serverless off and the replica count at one.
5. Wait for Railway to report `GET /healthz` healthy.
6. In Railway **Variables**, reveal and save each generated key in an approved
   secret manager. Label them clearly; they are not interchangeable.
7. Open `https://<your-domain>/management.html`.
8. Enter the management key when the panel asks for the management secret.
9. Add only provider accounts you are authorized to use. Follow the provider's
   OAuth/device-code flow shown by CLIProxyAPI.
10. Configure Codex or another compatible API client with the Railway HTTPS
    domain as its base URL and the proxy key as its Bearer credential.

Do not paste either key into a URL, source file, issue, screenshot, shell
history, or public client configuration. A browser or desktop client that
stores the key is inside your trust boundary.

## Keys and authentication

| Variable | Template value | Use |
|---|---|---|
| `CLIPROXY_PROXY_KEY` | `${{secret(64)}}` | Bearer credential for `/v1/*` proxy requests |
| `CLIPROXY_MANAGEMENT_KEY` | `${{secret(64)}}` | Administration credential for `/v0/management/*` and Management Center |

The template editor must serialize two independent generators. The wrapper
rejects missing, short, malformed, placeholder-like, or equal values and exits
without starting the public listener.

Example client request:

```sh
curl https://YOUR_DOMAIN/v1/models \
  -H "Authorization: Bearer YOUR_PROXY_KEY"
```

Use placeholders only in documentation. Never run the example with a real key
in a shared terminal history. Missing or wrong proxy credentials return `401`.
The management key must not authenticate the proxy API, and the proxy key must
not authenticate management routes.

## Add provider accounts remotely

The bundled Management Center is served at `/management.html`; it is pinned in
the image so startup and first access never download an unverified panel.
Authenticate the panel with the management key, choose the supported provider,
start its authorization flow, and complete the browser or device-code step.
The resulting provider state persists under `/data/auth`.

Provider flows change independently. Some require a paid subscription, local
browser confirmation, device code, callback, organization policy, or explicit
automation permission. This template does not promise that every subscription
or provider works, that a provider permits a particular use, or that access is
free of usage charges. Review the upstream
[provider documentation](https://github.com/router-for-me/CLIProxyAPI#readme)
and provider terms before connecting an account.

## Persistence

The volume boundary is `/data`:

- `/data/auth`: provider authorization material and account files.
- `/data/home`: the rootless runtime home.
- `/data/state`: package-reserved persistent configuration, including the two
  template access credentials in `/data/state/config.yaml`.
- `/data/update`: mode-`0700` updater ledger, embedded fallback, current and
  prior verified binaries, transient staged candidate, quarantine decisions,
  cadence timestamps, ETag, and crash-recovery phase. Files are bounded and
  written with restrictive modes and atomic `fsync` + rename.

Railway ext4 volumes may expose `/data/lost+found`. That directory is
filesystem-maintenance metadata, may remain root-owned, and is not CLIProxyAPI
application state. Do not delete, repurpose, recursively chown, or make it
world-readable. The wrapper ignores it.

## Backup and restore

Provider authorization files and the persistent configuration are sensitive
credentials. Store every backup encrypted, restrict access, and apply the
providers' security requirements.

1. Stop the service or otherwise ensure CLIProxyAPI is not writing.
2. Archive the complete app-owned `/data/auth`, `/data/home`, `/data/state`,
   and `/data/update` trees with paths, ownership, modes, and hidden files.
3. Do not include or modify `/data/lost+found`; record it as excluded
   filesystem metadata.
4. Record the CLIProxyAPI tag/digest, Management Center version/checksum,
   archive checksum, and backup time.
5. Restore into a fresh mounted `/data` while the service is stopped.
6. Ensure the volume root is pristine `root:root 0755` or already initialized
   `10001:10001 0750`; do not weaken it to make a restore work.
7. Start the last proved image and verify health, proxy auth separation,
   management auth, panel checksum, provider account presence, and one
   non-sensitive provider request.

A live file copy is not claimed to be consistent. Never attach the same volume
to multiple replicas. If authorization material may have leaked, revoke it at
the provider and reauthorize rather than trusting the archive. If
`/data/state/config.yaml` may have leaked, rotate both Railway template keys
after restore; provider revocation is still separate.

## Runtime stable-release update and rollback

Every deployed instance owns its update cycle; Railway or repository
redeployment is not required:

1. On boot it checks immediately when the persisted schedule is absent,
   overdue, or implausibly more than 23 hours in the future. While running it
   checks every 6 hours plus deterministic per-installation jitter of at most
   30 minutes. Transient retries are capped so a continuously running instance
   attempts within every rolling 24 hours.
2. It queries only `router-for-me/CLIProxyAPI` GitHub Releases and accepts the
   highest numeric, exact `vMAJOR.MINOR.PATCH` release that is non-draft and
   non-prerelease. Major versions pass the same gates; there is no silent major
   hold. A release soaks for 6 hours, within the 12-hour safety ceiling.
3. It downloads exactly `checksums.txt` and the one matching
   `linux_amd64`/`linux_aarch64` archive over an HTTPS/final-host allowlist.
   It bounds metadata, asset count, names and sizes, verifies the exact SHA-256
   line, rejects checksum reuse drift, and stream-extracts only the expected
   regular executable. Links, traversal, duplicate names and tar-shape drift
   fail closed.
4. It stages with `O_NOFOLLOW`, safe modes, an advisory single-updater lock,
   atomic `fsync` + rename, and a persistent phase journal. A private candidate
   starts on loopback with disposable state and proves its exact version,
   readiness, proxy/management credential separation, bundled UI, and clean
   exit. The upstream control plane is never exposed.
5. A passing candidate replaces only the executable. Live
   `/data/auth`, `/data/home`, and `/data/state` remain in place, so the updater
   never restores stale user data or discards writes. Readiness and bounded
   probation failure automatically restore the prior verified binary.
6. The embedded image binary remains an immutable fallback. Storage retains
   embedded, current, prior, and at most one staged candidate. A deterministic
   bad tag is quarantined instead of downloaded repeatedly.

The ledger records attempt start, last success, next check, observed ETag,
accepted checksums, sanitized failure class/reason, phase and exact
`tag@checksum` quarantine under `/data/update`; it never contains
provider, proxy, management, Railway, or GitHub credentials. Logs report only
sanitized outcomes.

Binary-only rollback cannot guarantee compatibility with an upstream release
that irreversibly migrates persisted provider state. The candidate probe uses
disposable state, but it cannot predict every real provider/account behavior.
Keep encrypted stopped backups for critical installations, and restore user
state only as an explicit operator recovery action.

The external GitHub Actions release controller remains a build-time
qualification/canary layer for future embedded fallbacks. It is not the
consumer update mechanism.

Independent R7 QA accepted the exact runtime package and its release, redirect,
archive, recovery, authentication, and rollback fixtures. Separate Railway
proofs then observed an overdue boot check promote `v7.2.141` to the verified
fixture `v7.2.142`, retain it across the sole service restart, and, in a fresh
rollback proof, quarantine an exact bad-live `v7.2.143` after authenticated
live semantic validation failed while automatically restoring healthy
`v7.2.141`. Those bounded fixtures prove the mechanism, not compatibility with
every future upstream release or state migration.

## Operations and troubleshooting

- Health: `GET /healthz` returns only `ok`.
- Proxy API: `/v1/*`, authenticated with the proxy key.
- Management API: `/v0/management/*`, authenticated with the management key.
- Management UI: `/management.html`, checksum-pinned in the image.
- Logs: retrieve a bounded interval and redact keys, Authorization headers,
  OAuth codes, cookies, account identifiers, prompts, model content, private
  domains, and provider responses.
- `secure initialization failed`: repair the missing/malformed/equal keys or
  unsafe `/data` ownership; never weaken validation.
- `401` on `/v1/models`: use the proxy key, not the management key.
- `401` in the panel: use the management key and confirm it was copied exactly.
- Provider login fails: check provider status/terms and upstream behavior;
  template support cannot grant or recover provider access.
- State disappears: stop writes and confirm the intended volume is mounted at
  `/data` before reauthorizing accounts.

See [docs/operations.md](docs/operations.md) and
[docs/environment.md](docs/environment.md) for the full runbooks.

## Security and support boundaries

This is a trusted-operator gateway. Anyone with the proxy key can consume the
connected provider access; anyone with the management key can change
configuration and authorization. It is not an untrusted public relay,
multi-tenant isolation boundary, credential-sharing service, provider reseller,
or authorization bypass.

Open a [template issue](https://github.com/l4time/railway-cliproxyapi-template/issues/new/choose)
for reproducible defects in the wrapper, digest/controller, generated-key
wiring, port/health, `/data` mount, pinned panel, or these instructions. Use
[upstream issues](https://github.com/router-for-me/CLIProxyAPI/issues) for
CLIProxyAPI behavior and [Railway support](https://railway.com/help) for
account, billing, domain, build, volume, or platform incidents. Report package
vulnerabilities through a
[private security advisory](https://github.com/l4time/railway-cliproxyapi-template/security/advisories/new).

## License and trademarks

Package code and documentation are MIT licensed. CLIProxyAPI and the Management
Center are MIT-licensed upstream projects and retain their own copyright,
dependency, and notice obligations. Provider and product names belong to their
owners. See [NOTICE.md](NOTICE.md), [SECURITY.md](SECURITY.md), and
[TRADEMARKS.md](TRADEMARKS.md).

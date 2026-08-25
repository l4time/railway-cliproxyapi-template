# CLIProxyAPI — Secure Release-Tracked

Deploy [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) as one
digest-pinned Railway service with separate generated proxy and administration
keys, a persistent `/data` volume, and a checksum-pinned Management Center.

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
- Management Center `v1.22.6`, bundled at build time and verified with SHA-256
  `e2643e08…2b721f4`.

There is no database, Redis, Bucket, worker, scheduler, provider account,
provider token, shared credential, runtime updater, or Railway token in the
application. Serverless is off, there is one replica, and failed processes get
at most 10 automatic retries under Railway's finite `ON_FAILURE` policy.

## Architecture

```text
API client -- Bearer proxy key ------+
                                     |
Browser -- pinned /management.html --+--> Railway HTTPS :8080
                                            |
                                  rootless health/reverse proxy
                                            |
                                  CLIProxyAPI 127.0.0.1:8317
                                            |
                         /data/auth  /data/home  /data/state
```

The root entrypoint performs only the narrow ownership/configuration bootstrap
needed for a fresh Railway volume. It validates both keys, initializes the
three app-owned directories, writes a mode-`0600` ephemeral config under
`/run`, removes source keys from the child environment, and drops permanently
to UID/GID `10001`. CLIProxyAPI is never bound directly to the public port.

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
- `/data/state`: package-reserved persistent state.

Railway ext4 volumes may expose `/data/lost+found`. That directory is
filesystem-maintenance metadata, may remain root-owned, and is not CLIProxyAPI
application state. Do not delete, repurpose, recursively chown, or make it
world-readable. The wrapper ignores it.

## Backup and restore

Provider authorization files are sensitive credentials. Store every backup
encrypted, restrict access, and apply the providers' security requirements.

1. Stop the service or otherwise ensure CLIProxyAPI is not writing.
2. Archive the complete app-owned `/data/auth`, `/data/home`, and
   `/data/state` trees with paths, ownership, modes, and hidden files.
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
the provider and reauthorize rather than trusting the archive.

## Release tracking, update, and rollback

The application never self-updates. A repository GitHub Actions controller:

1. Inspects official upstream GitHub Releases, accepting only non-draft,
   non-prerelease `vMAJOR.MINOR.PATCH` tags.
2. Waits at least 12 hours after publication.
3. Resolves the matching Docker image to an immutable manifest digest.
4. Requires the candidate numeric semantic version to be greater than the
   current version; equal is a no-op only for the same digest, while lower or
   equal-tag/different-digest candidates fail closed.
5. Refuses duplicate, rollback-target, malformed, or more-than-one-per-24-hour
   promotions.
6. Builds the candidate and runs the full key/auth/UI/state/restart/version
   transition smoke.
7. Atomically refreshes and verifies the Dockerfile entry in
   `SOURCE_SHA256SUMS`, then commits only the tested Dockerfile, release ledger,
   and checksum record.
8. Retains the previous tag/digest for a tested manual rollback.

It uses only the repository-scoped GitHub token; it has no Railway token and
does not copy application/provider secrets. A linked Railway source deployment
may build the accepted commit automatically, but this package is named
“Secure Release-Tracked” until clean consumer evidence proves Railway's update
serialization and rollback behavior. Do not rename it or market it as an
automatic-update product before that gate passes.

Before an update, make an encrypted stopped backup. Rollback changes the image
pin, not provider-side tokens or persisted state. If the older image cannot
safely open current state in a disposable copy, restore the pre-update backup
together with the last proved image.

Emergency rollback validates only the retained rollback target. It does not
build or boot the outgoing image, which may be broken. The target still must
pass the complete auth, pinned-UI, state, restart, child-failure, secret, and
resource smoke; any target failure stops before commit.

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

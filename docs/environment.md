# Environment Variable Guide

## Required generated secrets

| Variable | Exact Railway expression | Description shown to deployer |
|---|---|---|
| `CLIPROXY_PROXY_KEY` | `${{secret(64)}}` | Generated Bearer key for API clients. Save privately; do not use for administration. |
| `CLIPROXY_MANAGEMENT_KEY` | `${{secret(64)}}` | Generated administration key for the pinned Management Center and management API. Save separately; never give to normal API clients. |

Create two distinct variable rows in the Railway template editor. Each must be
generated and hidden, with no prompt, literal, example, or shared reference.
The clean consumer smoke must prove they serialize as different 64-hex values
and remain absent from public metadata and logs.

The runtime accepts a conservative URL-safe character set and rejects values
shorter than 32 characters, common placeholders, whitespace, and equality.
The public template contract is stricter: both values come from independent
`${{secret(64)}}` generators.

## Fixed/runtime values

| Variable | Rule |
|---|---|
| `PORT` | Railway supplies the public port; the package default is `8080`. The template/domain target is `8080`. |
| `MANAGEMENT_STATIC_PATH` | Image-owned `/opt/cliproxy/management.html`; do not override. |
| `HOME` | Entrypoint-owned `/data/home`; do not set in the template. |

Do not add CLIProxyAPI YAML settings as ad-hoc environment variables. The
entrypoint creates a mode-`0600` ephemeral configuration that binds upstream to
loopback, enables authenticated remote management, disables panel self-update,
sets `/data/auth`, disables file logging/statistics, and injects only the
validated keys before removing them from the child environment.

## Provider credentials

The template has no provider credential variables. Add accounts through the
pinned Management Center and complete the provider's supported authorization
flow. Never put OAuth tokens, cookies, subscription credentials, refresh
tokens, or account exports into Railway template defaults or repository files.

Provider authorization state under `/data/auth` is secret material. Treat a
volume backup as a credential backup.

## Rotation

- Proxy key: changing the Railway variable and redeploying changes client
  access. Update trusted clients, verify the new key, and revoke/remove the old
  value from secret managers and client stores.
- Management key: rotate during a maintenance window, verify panel and
  management API access with the new value, and remove the old value.
- Provider authorization: revoke or rotate at the provider, then reauthorize
  through the panel. Changing a template key does not revoke provider tokens.

Never rotate both template keys and all provider authorization simultaneously
without a verified recovery path.

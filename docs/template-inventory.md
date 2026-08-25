# CLIProxyAPI Template Packaging Inventory

Status: published package under narrow management-persistence correction.
Correction builder and local regression are complete; independent correction
QA, exact repository sync, fresh public consumer smoke, and monitoring intake
remain separate gates.

## Package

| Field | Answer |
|---|---|
| Candidate | CLIProxyAPI — Secure Release-Tracked |
| Proposed code | `cliproxyapi-secure-release-tracked` |
| Product-kit version | `2026-07-04-v1` |
| Upstream | `router-for-me/CLIProxyAPI` `v7.2.141`, MIT |
| Runtime pin | `eceasy/cli-proxy-api@sha256:7f598ce6…bdb3def` |
| Management UI | `Cli-Proxy-API-Management-Center` `v1.22.6`, SHA-256 `e2643e08…2b721f4`, MIT |
| Architecture | Single-service secure gateway plus volume |
| Service / volume | One `cliproxyapi`; one `/data` |
| Public port / health | `8080`; `/healthz` |
| Replicas / Serverless / restart | One / off / ON_FAILURE, maximum 10 retries |
| Extra service | None |

## Variables

| Variable | Kind | Exact value / rule |
|---|---|---|
| `CLIPROXY_PROXY_KEY` | Generated secret | `${{secret(64)}}`; API clients only; no prompt/default |
| `CLIPROXY_MANAGEMENT_KEY` | Generated secret | `${{secret(64)}}`; administration only; separate generator; no prompt/default |
| `PORT` | Runtime/default | Railway target `8080`; no consumer input |

No provider credential, subscription, account, shared secret, database
connection, Railway token, GitHub token, or runtime updater variable exists.

## Repository assets

| Surface | File(s) | Builder status |
|---|---|---|
| Runtime | `Dockerfile`, `entrypoint.sh`, `config-reconciler.go`, `health-proxy.go`, `.dockerignore` | Persistence R2 reasserts canonical security fields, uses a strict grammar/allowlist, and corrects the health default; full re-proof required |
| Railway config | `railway.json`, contract | Complete |
| Release automation | workflow, controller, state, `SOURCE_SHA256SUMS` | Complete; transactional checksum refresh, external only |
| Tests | `tests/run.sh`, container/static/controller tests | Complete |
| Deploy/user path | `README.md`, `overview.md`, environment guide | Complete |
| Operations/security | operations, architecture/security, `SECURITY.md` | Complete |
| Update/rollback | release-controller guide, changelog | Complete |
| Support intake | issue form/routes, labels, CONTRIBUTING, CODEOWNERS | Complete |
| Legal | `LICENSE`, `NOTICE.md`, `TRADEMARKS.md` | Complete |
| Marketplace | marketplace overview, assets exception | Complete; exception pending QA |
| Inventory/lifecycle | this file, completion packet, QA checklist | Builder complete; independent gates pending |

## Support ownership

Package owner:

- Wrapper, root drop, pinned panel, health proxy, Dockerfile and release ledger.
- Railway variable, port/domain/health, volume, replica, Serverless, and
  restart contract.
- Release-controller policy/tests and package documentation.
- Reproducible clean-deploy, auth-separation, persistence, and rollback defects.

Other owner:

- CLIProxyAPI/Management Center application behavior: upstream.
- Provider account, terms, subscription, authorization, quotas, models,
  privacy, billing, and uptime: user/provider.
- Client storage/configuration beyond documented baseline: user/client.
- Railway account, billing, domain, volume, build, or platform: Railway.

## Gate state

| Gate | State |
|---|---|
| Local preflight / corrected Railway R2 | Passed; exact refs in build notes |
| Build approval | Passed by canonical work order |
| Builder package | Complete |
| Builder local regression | Persistence R2 PASS: static 15/15; separate Go vets; full forward and isolated rollback-target; strict parser, malicious drift/no-wildcard, allowlist, rotation, restart; zero owned residue |
| Independent QA | Original package passed; persistence correction re-QA pending |
| Public repository | Published at `db1f335d…`; exact correction sync pending |
| Draft / draft smoke | Historical PASS; no replacement draft authorized by this builder |
| Publish approval / publication | Published; correction remains blocked from acceptance |
| Clean public consumer smoke | Latest FAIL isolated to now-corrected management persistence; fresh retest pending |
| Cleanup / monitoring intake | All failed-smoke resources deleted; monitoring acceptance pending corrected smoke |

## Known limits

- One trusted-operator service and one replica only.
- Provider authorization is sensitive and provider-policy dependent.
- No per-user RBAC, tenant isolation, abuse controls, cost/provider guarantees,
  HA, or Serverless support.
- Management Center updates are manual and separately reviewed.
- Persistent `/data/state/config.yaml` contains both template credentials; all
  backups are credential backups, and restored stale keys are reconciled from
  the current Railway variables at boot.
- Only `debug`, `request-retry`, `max-retry-credentials`, and
  `max-retry-interval` persist as user-managed non-secret settings. Wrapper
  security/topology is canonical and cannot be overridden by persisted YAML.
- Release automation commits a tested source pin; consumer update inheritance
  remains unclaimed until clean public evidence.
- Automated promotion is forward-only; emergency rollback validates only the
  retained target and does not prove arbitrary historical downgrades.
- Release mutation requires an exact, current four-record source checksum
  manifest; malformed/missing/stale checksum state fails closed.
- Asset exception remains subject to QA and publish approval.

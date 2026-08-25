# CLIProxyAPI Template Packaging Inventory

Status: repository-ready 42-file Daily Auto-Update package. Independent R7 QA
and the combined Railway promotion/restart and bad-live rollback proofs are
accepted. Exact public repository/template synchronization and a clean consumer
smoke of the synchronized package remain separate lifecycle gates.

## Package

| Field | Answer |
|---|---|
| Candidate | CLIProxyAPI — Daily Auto-Update |
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
connection, Railway token, GitHub token, or runtime-updater variable exists.
The built-in updater needs no consumer input or credential.

## Repository assets

| Surface | File(s) | Builder status |
|---|---|---|
| Runtime | `Dockerfile`, `entrypoint.sh`, `config-reconciler.go`, `health-proxy.go`, `.dockerignore` | Persistence R2 plus rootless supervisor, official stable-release verifier, private probe, cutover/rollback, bounded `/data/update` ledger |
| Railway config | `railway.json`, contract | Complete |
| Release automation | runtime supervisor plus workflow, controller, state, `SOURCE_SHA256SUMS` | Runtime is the consumer mechanism; external controller remains embedded-fallback canary |
| Tests | `health-proxy_test.go`, `tests/run.sh`, container/static/controller tests | Runtime unit/race/vet, static, recovery, adversarial updater, auth/state/UI and rollback suites PASS; independent R7 QA ACCEPT |
| Deploy/user path | `README.md`, `overview.md`, environment guide | Complete |
| Operations/security | operations, architecture/security, `SECURITY.md` | Complete |
| Update/rollback | runtime supervisor, operations, release-controller guide, changelog | Independent R7 QA ACCEPT; Railway Phase A promotion/restart and fresh-project bad-live rollback/quarantine PASS |
| Support intake | issue form/routes, labels, CONTRIBUTING, CODEOWNERS | Complete |
| Legal | `LICENSE`, `NOTICE.md`, `TRADEMARKS.md` | Complete |
| Marketplace | marketplace overview, assets exception | Complete; no-custom-asset exception accepted for the existing listing |
| Inventory/lifecycle | this file, completion packet, QA checklist | Complete for the repository-ready 42-file package; public sync/smoke remain external |

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
| Builder local regression | Current updater package PASS: static 16/16, Go race/vet, both architecture builds, auth/state/UI/restart/rollback and native updater fixtures; zero owned residue |
| Independent QA | R7 exact digest `a5a25a49…` ACCEPT with zero P0-P3; native updater matrix peaked at 7.758 MiB |
| Railway updater proof | Phase A PASS: overdue `v7.2.141 → v7.2.142` promotion and sole-service restart persistence. Fresh R3 PASS: bad-live `v7.2.143` live-semantic rejection, exact quarantine, and automatic healthy `v7.2.141` restore; sanitized logs/resources passed |
| Public repository | Existing published source remains at `b9617b71…`; exact 42-file Daily Auto-Update sync is the next external gate |
| Draft / draft smoke | Historical listing PASS; synchronized update draft review remains external |
| Publish approval / publication | Existing template is published under stable code `cliproxyapi-secure-release-tracked`; source/metadata update remains external |
| Clean public consumer smoke | Original protected-config package passed; synchronized Daily Auto-Update consumer smoke remains required |
| Cleanup / monitoring intake | R1/R2/R3 disposable projects, domains, volumes, fixtures, captures and scheduled inventories are fully cleared; update monitoring intake follows synchronized smoke |

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
- Runtime update checks every 6 hours plus at most 30 minutes jitter and accepts
  only newer stable exact semver after a 6-hour soak and
  checksum/archive/private-probe/live-semantic gates. An overdue boot checks
  immediately; a continuously running service attempts within every rolling
  24 hours.
- Rollback is binary-only against live protected state; irreversible upstream
  state migration still requires an explicit stopped backup restore.
- Automated promotion is forward-only; emergency rollback validates only the
  retained target and does not prove arbitrary historical downgrades.
- Release mutation requires an exact, current five-record source checksum
  manifest; malformed/missing/stale checksum state fails closed.
- The accepted no-custom-asset exception applies to the current listing; any
  future custom asset needs original provenance.

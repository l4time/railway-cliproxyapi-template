# Template Product Kit Completion Packet

| Field | Answer |
|---|---|
| Candidate / proposed code | CLIProxyAPI — Daily Auto-Update / `cliproxyapi-secure-release-tracked` |
| Kit version | `2026-07-04-v1` |
| Archetype | Single-service secure gateway plus one `/data` volume |
| Completion owner | `cliproxy_template_builder`; main thread integrates |
| Runtime evidence | Independent R7 QA ACCEPT on exact digest `a5a25a49…`; Railway Phase A proved unattended promotion/restart and fresh R3 proved bad-live authenticated semantic rejection, exact quarantine, and automatic prior restart |
| Required kit pieces | Deploy/setup, variables, operations, support, changelog/update, marketplace, issue/labels, legal/security/trademark, inventory, QA — builder complete |
| Release value | Credential-free runtime stable-release updater with bounded cadence/soak, official checksums, strict archive parser, private probe, journal/quarantine and binary-only rollback; external controller remains embedded-fallback canary |
| Provider boundary | User-authorized accounts only; no shared credentials, auth bypass, included subscription, provider permission, or free-use promise |
| Asset status | Explicit initial no-custom-asset exception; no copied logo or credential-bearing screenshot |
| QA blocker status | Closed for the repository-ready package: independent R7 QA accepted zero P0-P3 and combined Railway R2/R3 updater proof passed |
| Publish readiness | Repository-ready 42-file update; exact repository/template synchronization and a fresh clean public consumer smoke remain external gates |
| Monitoring intake | Prepared in docs; dashboard rows remain main-thread work after publication |
| Main-thread memory targets | Build/Publish Work Order, Product Kit Adoption, Active Subagent Registry closure, QA result, draft/publish/smoke/cleanup, published registry and monitoring contract |
| Compaction | Collapse detailed build evidence to ledger/file refs after the 30-day review unless a defect remains |

## Piece-level status

| Piece | Artifact | Builder | Independent |
|---|---|---|---|
| Non-expert deploy and remote OAuth | README, environment guide | Complete | Accepted in original public smoke; updater wording reconciled |
| Runtime/config and key separation | Dockerfile, config reconciler, scripts, contract | Complete | Accepted; rechecked through Railway update/restart/rollback |
| Persistence, backup, lost+found | README, operations | Complete | Accepted with explicit no-auto-rewind migration limit |
| Runtime update/rollback | supervisor, persistent ledger, private probe, operations, unit fixtures | Complete | R7 QA plus Railway R2/R3 PASS |
| External fallback qualification | workflow, controller, ledger, source checksum, guide | Complete | Accepted as build-time canary, not consumer updater |
| Provider/legal/support boundary | README, security, notices, issue routes | Complete | Accepted; no provider-access promise |
| Marketplace identity/copy | overview and marketplace overview | Complete | Repository-ready; public metadata sync external |
| Assets | Explicit no-custom-asset exception | Complete | Accepted for existing listing |
| Tests/evidence | static/controller/container suites, build notes | Complete | Independent R7 ACCEPT; live updater proof PASS |
| Inventory/lifecycle | inventory, completion, QA | Complete | 42-file repository package accepted; public sync/smoke external |

## Builder handoff

Repository-sync QA can reproduce:

```sh
SKIP_DOCKER_TESTS=1 tests/run.sh
docker run --rm -v "$PWD:/src" -w /src golang:1.25.5-bookworm \
  sh -c '/usr/local/go/bin/go test -race health-proxy.go health-proxy_test.go'
tests/run.sh full
tests/run.sh rollback-target
shasum -a 256 -c SOURCE_SHA256SUMS
```

The accepted evidence verifies promote and rollback refresh the Dockerfile checksum, malformed
or missing records fail without mutation, post-change verification passes, and
the workflow stages exactly Dockerfile, release state, and checksum. It must
also covered exact fixture origins, recovery phases, transient/deterministic
classification, authenticated live semantics, and binary-only rollback.
Repository-sync QA must still inspect default-branch/secret permissions,
validate public links, and reconcile real Railway draft variables. No package
text authorizes external repository, Railway, publish, or provider mutation by
the builder.

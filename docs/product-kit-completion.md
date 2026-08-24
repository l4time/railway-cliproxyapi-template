# Template Product Kit Completion Packet

| Field | Answer |
|---|---|
| Candidate / proposed code | CLIProxyAPI — Secure Release-Tracked / `cliproxyapi-secure-release-tracked` |
| Kit version | `2026-07-04-v1` |
| Archetype | Single-service secure gateway plus one `/data` volume |
| Completion owner | `cliproxy_template_builder`; main thread integrates |
| Runtime evidence | Entrypoint/health hashes preserved; Dockerfile changed only to pin the exact Go builder digest and was re-proved |
| Required kit pieces | Deploy/setup, variables, operations, support, changelog/update, marketplace, issue/labels, legal/security/trademark, inventory, QA — builder complete |
| Release value | External numeric-forward controller with soak/digest/smoke/daily gates, target-only rollback, and transactional source-checksum refresh; no app updater/token |
| Provider boundary | User-authorized accounts only; no shared credentials, auth bypass, included subscription, provider permission, or free-use promise |
| Asset status | Explicit initial no-custom-asset exception; no copied logo or credential-bearing screenshot |
| QA blocker status | All five independent re-QA findings implemented; final 14/14 static and full forward Docker PASS, prior target-only rollback PASS retained; final independent re-QA pending |
| Publish readiness | Not claimed; public repository, draft serialization, asset exception, links, publish packet, consumer smoke, cleanup, and monitoring pending |
| Monitoring intake | Prepared in docs; dashboard rows remain main-thread work after publication |
| Main-thread memory targets | Build/Publish Work Order, Product Kit Adoption, Active Subagent Registry closure, QA result, draft/publish/smoke/cleanup, published registry and monitoring contract |
| Compaction | Collapse detailed build evidence to ledger/file refs after the 30-day review unless a defect remains |

## Piece-level status

| Piece | Artifact | Builder | Independent |
|---|---|---|---|
| Non-expert deploy and remote OAuth | README, environment guide | Complete | Pending |
| Runtime/config and key separation | Dockerfile, scripts, contract | Complete | Pending |
| Persistence, backup, lost+found | README, operations | Complete | Pending |
| External update/rollback | workflow, controller, ledger, source checksum, guide | Complete | Pending |
| Provider/legal/support boundary | README, security, notices, issue routes | Complete | Pending |
| Marketplace identity/copy | overview and marketplace overview | Complete | Pending |
| Assets | Explicit no-custom-asset exception | Complete | Pending acceptance |
| Tests/evidence | static/controller/container suites, build notes | Complete; final builder PASS | Final re-QA pending |
| Inventory/lifecycle | inventory, completion, QA | Complete | Pending |

## Builder handoff

Independent QA should run:

```sh
SKIP_DOCKER_TESTS=1 tests/run.sh
tests/run.sh full
tests/run.sh rollback-target
shasum -a 256 -c SOURCE_SHA256SUMS
```

It must verify promote and rollback refresh the Dockerfile checksum, malformed
or missing records fail without mutation, post-change verification passes, and
the workflow stages exactly Dockerfile, release state, and checksum. It must
also inspect default-branch/secret permissions, validate public links, and
reconcile real Railway draft variables. No package text authorizes external
repository, Railway, publish, or provider mutation by the builder.

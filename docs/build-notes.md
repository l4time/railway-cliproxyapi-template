# Build and Proof Notes

## Accepted runtime source

The initial package copies the corrected local/Railway R2 source exactly:

| File | SHA-256 |
|---|---|
| `Dockerfile` | `6fa6c77c504b951784210ab13dc2e341ecb96d9d011469b791019e65b3c6275c` |
| `entrypoint.sh` | `39b3bd5d7e262f490172282d207a7563601d566ae52c8ba1064bbecf0fab0c77` |
| `health-proxy.go` | `dbc811f7b49110392ca3db0870a53fb939c0b51e86be576721a18549fb935c32` |

The earlier proved Dockerfile hash was
`cc534683f850551f8c26f635ba99c507ac0b1024680a6e2cb193ef9e3b52c01e`.
Independent QA required the Go builder stage to use immutable digest
`sha256:d9132cce84391efab786495288756d60e1da215b1f94e87860aeefc3d4c45b6d`;
that one reproducibility-only textual change produced the new Dockerfile hash
above. Entrypoint and health-proxy bytes, final runtime code, UID, configuration,
and behavior are unchanged and were regression-tested again.

Future release promotion necessarily changes the two upstream Dockerfile digest
lines. The entrypoint and health proxy remain byte-locked unless a separately
reviewed runtime change receives new proof.

## Initial pins

- CLIProxyAPI stable release: `v7.2.141`.
- Current image index digest:
  `sha256:7f598ce64478a8a5f90ed76875e0e9b0e7d77b80e17184b13df18c3d5bdb3def`.
- Prior tested image index digest (`v7.2.140`):
  `sha256:87c0bc86d4a8d6a5aff670bbdfea18bfe28decfee0888924301b502d57fe303e`.
- Management Center: `v1.22.6`, SHA-256
  `e2643e0875e0024e5ff9ddf4569e4c58611ab0456aeb6fa6065ed3e6c2b721f4`.

## Composite proof boundary

Local and corrected Railway R2 evidence covered one service/domain/volume,
separate key authentication, pinned UI, same-volume
`7.2.140 → 7.2.141 → 7.2.140 → 7.2.141`, restart persistence, rootless
steady state, bounded logs/resources, and complete resource cleanup. R2 peak
was about 29.49 MB and 0.00618 vCPU without provider traffic.

Those observations are not cost, capacity, performance, update compatibility,
provider-access, uptime, or future-version guarantees.

## Build dependencies

- Docker with BuildKit and network access to Docker Hub and GitHub Releases.
- `golang:1.25.5-bookworm@sha256:d9132cce84391efab786495288756d60e1da215b1f94e87860aeefc3d4c45b6d`
  builder stage.
- Exact upstream runtime manifest.
- Exact Management Center release asset plus Dockerfile `ADD --checksum`.

The final image contains the upstream runtime, statically built health proxy,
pinned panel, entrypoint, and UID/GID `10001`. Product-kit files and tests are
excluded by `.dockerignore`.

## Local verification

Run:

```sh
SKIP_DOCKER_TESTS=1 tests/run.sh
tests/run.sh full
tests/run.sh rollback-target
```

The first command runs policy/static tests. The second adds the complete
forward transition matrix. The third builds only the retained rollback target
and repeats its auth/state/UI/restart/failure/resource gates without touching
the outgoing image. Every container run cleans all local objects through a
trap; verify zero matching containers, images, networks, and volumes after each.

Promotion and rollback also validate that `SOURCE_SHA256SUMS` has exactly the
three ordered records, that its pre-change Dockerfile hash is current, and that
the post-change hash verifies. Dockerfile, release state, and checksum content
are fully prepared before per-file atomic replacement. This is not a
cross-file transaction: caught errors restore originals, but a hard
runner/host crash can leave only an uncommitted partial tree. The workflow
cannot push it, and the next run fails closed on checksum drift. Ephemeral
GitHub-hosted runners discard it; reused/self-hosted runners must reset to the
last trusted commit before rerunning.

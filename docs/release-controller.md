# External Release Controller

## Purpose

The external controller turns upstream release availability into a small,
auditable, fail-closed embedded-fallback source change. It runs in GitHub
Actions, never receives Railway or provider credentials, and remains a
build-time qualification/canary defense. Deployed consumers update through the
separate credential-free runtime supervisor documented in the README.

## Scheduled promotion

The workflow runs every six hours but can promote at most once in 24 hours.
It queries `router-for-me/CLIProxyAPI` releases, selects the newest stable exact
semantic tag, checks its publication time, resolves the same tag from
`eceasy/cli-proxy-api` to the SHA-256 of the raw registry manifest, and asks
`scripts/release_controller.py` for a decision. Mutation is allowed only from
the repository's configured default branch. Numeric `(major, minor, patch)`
comparison must prove the candidate is greater than current; lexical string
ordering is never used.

Possible decisions:

| Decision | Meaning |
|---|---|
| `promote` | Stable, soaked, new immutable digest and daily limit clear |
| `defer:soak` | Release is less than 12 hours old |
| `defer:daily-limit` | Another promotion occurred less than 24 hours ago |
| `noop:current` | Digest is already running |
| `noop:current-digest` | A higher tag resolves to the already-running digest |
| `reject:draft` / `reject:unstable-tag` | Release shape is outside policy |
| `reject:digest` | Immutable digest shape is invalid |
| `reject:non-forward` | Candidate semantic version is lower than current |
| `reject:current-tag-digest-drift` | Current tag was reused with a different digest |
| `reject:known-rollback` | Candidate matches the retained rollback target |
| `reject:future-release` | Publication timestamp is impossible |

Only `promote` modifies the working tree. The controller changes exactly the
two accepted Dockerfile pin lines, moves the previous current record into
`prior`, and refreshes the Dockerfile record in `SOURCE_SHA256SUMS`.
The checksum file must contain exactly five ordered, exact-format records and
must verify the pre-change Dockerfile. Missing, malformed, duplicate, reordered,
or stale records fail before mutation. Updated Dockerfile, state, and checksum
content are prepared together. Each file is atomically replaced; a caught
write or verification error restores all originals, and post-change
verification is mandatory.

This is not a cross-file filesystem transaction. A hard runner or host crash
between per-file replacements can leave a partial, uncommitted working tree.
The workflow cannot commit or push that state; the next run fails closed when
the recorded Dockerfile checksum does not match. GitHub-hosted ephemeral
runners discard the workspace. On a reused/self-hosted runner, reset the
uncommitted tree to the last trusted commit before rerunning.

## Smoke before commit

The workflow builds current and prior wrappers, uses random ephemeral keys,
and tests:

- Key validation and equality rejection.
- Proxy missing/wrong/correct authentication.
- Management missing/wrong/correct/cross-key authentication.
- Bundled panel body checksum and absence of download fallback.
- Rootless PID 1, directory/config ownership and modes.
- Secret absence from image history, arguments, environment, logs, and health.
- Prior/current/prior/current transitions on the same volume.
- State marker persistence and container restart.
- Child failure propagation.
- Bounded container resources.
- Release-controller fixtures and static contract.

Random values are held in mode-`0600` temporary files and removed with all
containers, images, network, and volume on exit. Shell tracing is not enabled.

## Commit and deploy boundary

After smoke, GitHub commits only `Dockerfile`, `release-state.json`, and
`SOURCE_SHA256SUMS`.
Repository branch protection and Railway source deployment remain external
controls. Accepted commits refresh the immutable embedded fallback; they are
not required for the deployed runtime updater to accept a verified newer
official stable executable.

## Rollback

Manual workflow input `rollback` swaps current and prior, updates both
Dockerfile pins, and runs a target-only complete smoke before committing.
Rollback mode builds and boots the retained target only; the outgoing image is
never required because it may be broken. The target must still pass key/auth,
pinned UI, state, restart, child-failure, secret-absence, and resource gates.
Any target failure makes the workflow fail before the success-gated commit.
Emergency rollback bypasses the forward-promotion 24-hour delay but does not
bypass tests. It does not mutate `/data`; operators must use a compatible
stopped backup if state cannot safely downgrade.

## Supply-chain limits

The workflow pins `actions/checkout` to an immutable commit. It does not follow
an upstream branch or image `latest`, execute upstream repository scripts, or
download an unverified Management Center at runtime. GitHub, Docker Hub, base
builder images, the upstream published image, and repository write access
remain trusted supply-chain dependencies.

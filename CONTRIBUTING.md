# Contributing

Contributions are welcome for reproducible defects in this package's Railway
contract, wrapper, release controller, tests, documentation, and support
surfaces.

Before opening a pull request:

1. Use a branch and keep the change scoped.
2. Never commit real template keys, provider credentials, account exports,
   Railway identifiers/tokens, private domains, prompts, responses, or volume
   data.
3. Run `SKIP_DOCKER_TESTS=1 tests/run.sh`.
4. For runtime, pin, panel, or controller changes, also run `tests/run.sh` with
   Docker available.
5. Update `CHANGELOG.md`, relevant contract/operations docs, and evidence
   boundaries.
6. Explain why the change belongs to the package rather than upstream
   CLIProxyAPI, Management Center, a provider, or Railway.

Runtime changes must preserve fail-closed key validation, loopback upstream,
rootless steady state, pinned panel, one-volume persistence, and secret
absence. Upstream version updates go through the release controller; do not
submit `latest`, branch, prerelease, tag-only, or smoke-skipping changes.

Use a private advisory, not a pull request or public issue, for security
reports.

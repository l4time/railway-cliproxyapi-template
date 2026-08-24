#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"
MODE=${1:-full}
case "$MODE" in
  full|rollback-target) ;;
  *) printf 'unsupported test mode: %s\n' "$MODE" >&2; exit 64 ;;
esac
python3 -m unittest discover -s tests -p 'test_*.py' -v
if command -v shellcheck >/dev/null 2>&1; then
  shellcheck entrypoint.sh tests/test_preflight.sh tests/run.sh
fi
if [ "${SKIP_DOCKER_TESTS:-0}" != "1" ]; then
  tests/test_preflight.sh "$MODE"
fi

#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
PREFIX=cliproxy-runtime-r2
IMAGE="${PREFIX}:fixture"
CONTAINER="${PREFIX}-app"
VOLUME="${PREFIX}-data"
PORT=18417
SERVER_PORT=18419
FIXTURE_ROOT=$(mktemp -d)
SERVER_PID=
HOST_UID=$(id -u)
HOST_GID=$(id -g)
PROXY_KEY=cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
MANAGEMENT_KEY=dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd

cleanup() {
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  docker volume rm "$VOLUME" >/dev/null 2>&1 || true
  docker image rm "$IMAGE" >/dev/null 2>&1 || true
  if [ -n "$SERVER_PID" ]; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  find "$FIXTURE_ROOT" -type f -delete 2>/dev/null || true
  rmdir "$FIXTURE_ROOT" 2>/dev/null || true
}
trap cleanup EXIT INT TERM
cleanup

case "$(docker info --format '{{.Architecture}}')" in
  arm64|aarch64) FIXTURE_ARCH=aarch64 ;;
  amd64|x86_64) FIXTURE_ARCH=amd64 ;;
  *) printf '%s\n' 'unsupported fixture architecture' >&2; exit 1 ;;
esac

build_candidate() {
  tag=$1
  docker run --rm \
    --user "${HOST_UID}:${HOST_GID}" \
    -e CGO_ENABLED=0 \
    -e FIXTURE_TAG="$tag" \
    -e HOME=/tmp/fixture-home \
    -e GOCACHE=/tmp/fixture-gocache \
    -v "$ROOT/tests/fake_candidate.go:/src/fake_candidate.go:ro" \
    -v "$FIXTURE_ROOT:/out" \
    -w /src \
    golang:1.25.5-bookworm \
    /bin/sh -eu -c \
    'mkdir -p "$HOME" "$GOCACHE" &&
      /usr/local/go/bin/go build -trimpath -ldflags="-s -w -X main.version=${FIXTURE_TAG}" -o /out/cli-proxy-api fake_candidate.go &&
      chmod 0755 /out/cli-proxy-api'
  COPYFILE_DISABLE=1 tar --format ustar -C "$FIXTURE_ROOT" -czf "$FIXTURE_ROOT/candidate.tar.gz" cli-proxy-api
  printf '%s\n' "$tag" > "$FIXTURE_ROOT/tag"
}

set_scenario() {
  printf '%s\n' "$1" > "$FIXTURE_ROOT/scenario"
}

force_overdue() {
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  docker run --rm -v "${VOLUME}:/data" busybox sed -i \
    's/"next_check": "[^"]*"/"next_check": "2000-01-01T00:00:00Z"/' \
    /data/update/ledger.json
}

start_app() {
  mode=$1
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  docker run -d \
    --name "$CONTAINER" \
    --add-host host.docker.internal:host-gateway \
    --cpus 0.5 \
    --memory 256m \
    --pids-limit 128 \
    --read-only \
    --tmpfs /run:rw,nosuid,nodev,noexec,size=8m,mode=0755 \
    --tmpfs /tmp:rw,nosuid,nodev,noexec,size=8m,mode=1777 \
    -p "127.0.0.1:${PORT}:8080" \
    -e CLIPROXY_PROXY_KEY="$PROXY_KEY" \
    -e CLIPROXY_MANAGEMENT_KEY="$MANAGEMENT_KEY" \
    -e CLIPROXY_UPDATER_FIXTURE=1 \
    -e CLIPROXY_UPDATER_FIXTURE_HOST=host.docker.internal \
    -e CLIPROXY_RELEASE_API="http://host.docker.internal:${SERVER_PORT}/releases" \
    -e CLIPROXY_FAKE_CANDIDATE_MODE="$mode" \
    -v "${VOLUME}:/data" \
    "$IMAGE" >/dev/null
}

wait_health() {
  tries=0
  until curl -fsS "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1; do
    tries=$((tries + 1))
    [ "$tries" -lt 100 ] || return 1
    sleep 0.2
  done
}

wait_tag() {
  want=$1
  tries=0
  until [ "$(docker exec "$CONTAINER" sed -n '/"current":/ {n;s/.*"tag": "\([^"]*\)".*/\1/p;}' /data/update/ledger.json)" = "$want" ]; do
    tries=$((tries + 1))
    [ "$tries" -lt 150 ] || {
      docker exec "$CONTAINER" sed -n '1,120p' /data/update/ledger.json >&2
      return 1
    }
    sleep 0.2
  done
}

wait_failure_class() {
  want=$1
  tries=0
  until docker exec "$CONTAINER" grep -F "\"last_failure_class\": \"$want\"" /data/update/ledger.json >/dev/null 2>&1; do
    tries=$((tries + 1))
    [ "$tries" -lt 100 ] || return 1
    sleep 0.2
  done
}

wait_idle() {
  tries=0
  until docker exec "$CONTAINER" grep -F '"phase": "idle"' /data/update/ledger.json >/dev/null 2>&1; do
    tries=$((tries + 1))
    [ "$tries" -lt 150 ] || return 1
    sleep 0.2
  done
}

docker build -t "$IMAGE" "$ROOT" >/dev/null
docker volume create "$VOLUME" >/dev/null
build_candidate 'v0.0.0$(touch>/out/fixture-injection)'
test ! -e "$FIXTURE_ROOT/fixture-injection"
printf '%s\n' 'fixture tag shell-injection boundary: PASS'
build_candidate v7.2.142
set_scenario good
FIXTURE_ROOT="$FIXTURE_ROOT" FIXTURE_PORT="$SERVER_PORT" FIXTURE_ARCH="$FIXTURE_ARCH" \
  python3 "$ROOT/tests/fake_release_server.py" >"$FIXTURE_ROOT/server.log" 2>&1 &
SERVER_PID=$!
sleep 0.5

start_app normal
wait_health
wait_tag v7.2.142
wait_idle
curl -fsS -H "Authorization: Bearer ${PROXY_KEY}" "http://127.0.0.1:${PORT}/v1/models" >/dev/null
curl -fsS -H "Authorization: Bearer ${MANAGEMENT_KEY}" "http://127.0.0.1:${PORT}/v0/management/config" >/dev/null
printf '%s\n' 'good promotion and live semantic validation: PASS'

docker restart "$CONTAINER" >/dev/null
wait_health
wait_tag v7.2.142
printf '%s\n' 'promoted binary restart persistence and equal-tag no-op: PASS'

build_candidate v7.2.143
set_scenario good
force_overdue
start_app bad-live
wait_health
wait_failure_class deterministic
wait_tag v7.2.142
wait_idle
printf '%s\n' 'live semantic failure automatic rollback: PASS'

docker restart "$CONTAINER" >/dev/null
wait_health
wait_tag v7.2.142
wait_idle
printf '%s\n' 'restart after live rollback before next check: PASS'

build_candidate v7.2.144
set_scenario transient
force_overdue
start_app normal
wait_health
wait_failure_class transient
if docker exec "$CONTAINER" grep -F 'v7.2.144@' /data/update/ledger.json >/dev/null; then
  printf '%s\n' 'transient acquisition was incorrectly quarantined' >&2
  exit 1
fi
printf '%s\n' 'transient acquisition remains retryable: PASS'

set_scenario bad-checksum
force_overdue
start_app normal
wait_health
wait_failure_class deterministic
docker exec "$CONTAINER" grep -F 'v7.2.144@' /data/update/ledger.json >/dev/null
printf '%s\n' 'bad checksum deterministic quarantine: PASS'

build_candidate v7.2.145
set_scenario good
printf '%s\n' 'not-a-gzip' > "$FIXTURE_ROOT/candidate.tar.gz"
force_overdue
start_app normal
wait_health
wait_failure_class deterministic
docker exec "$CONTAINER" grep -F 'v7.2.145@' /data/update/ledger.json >/dev/null
printf '%s\n' 'bad archive deterministic quarantine: PASS'

build_candidate v9.9.9
printf '%s\n' 'v7.2.146' > "$FIXTURE_ROOT/tag"
set_scenario good
force_overdue
start_app normal
wait_health
wait_failure_class deterministic
docker exec "$CONTAINER" grep -F 'v7.2.146@' /data/update/ledger.json >/dev/null
printf '%s\n' 'version mismatch private-probe quarantine: PASS'

build_candidate v7.2.147
set_scenario good
force_overdue
start_app bad-probe-auth
wait_health
wait_failure_class deterministic
wait_tag v7.2.142
wait_idle
docker exec "$CONTAINER" grep -F 'v7.2.147@' /data/update/ledger.json >/dev/null
printf '%s\n' 'correct-version private auth semantic failure quarantine: PASS'

build_candidate v7.2.142
set_scenario same-tag-drift
force_overdue
start_app normal
wait_health
wait_failure_class security
docker exec "$CONTAINER" grep -F 'v7.2.142@ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff' /data/update/ledger.json >/dev/null
printf '%s\n' 'same-tag checksum drift security quarantine: PASS'

docker exec "$CONTAINER" sh -c '
  test "$(awk "/^Uid:/ {print \$2}" /proc/1/status)" = 10001
  test "$(awk "/^NoNewPrivs:/ {print \$2}" /proc/1/status)" = 1
  test "$(awk "/^CapEff:/ {print \$2}" /proc/1/status)" = 0000000000000000
'
if docker logs "$CONTAINER" 2>&1 | grep -F "$PROXY_KEY" >/dev/null; then
  printf '%s\n' 'proxy key leaked to runtime logs' >&2
  exit 1
fi
if docker logs "$CONTAINER" 2>&1 | grep -F "$MANAGEMENT_KEY" >/dev/null; then
  printf '%s\n' 'management key leaked to runtime logs' >&2
  exit 1
fi
docker stats --no-stream --format 'runtime updater fixture resources: {{.MemUsage}} | {{.CPUPerc}}' "$CONTAINER"
printf '%s\n' 'ALL RUNTIME UPDATE FIXTURE GATES: PASS'

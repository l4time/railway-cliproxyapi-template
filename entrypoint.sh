#!/bin/sh
set -eu

APP_UID=10001
APP_GID=10001
DATA_DIR=/data
RUN_DIR=/run/cliproxy
CONFIG_FILE="${RUN_DIR}/config.yaml"

fail() {
  printf '%s\n' "secure initialization failed" >&2
  exit 64
}

valid_key() {
  key=$1
  [ "${#key}" -ge 32 ] || return 1
  case "$key" in
    *[!A-Za-z0-9._~-]*) return 1 ;;
    your-api-key*|your-secret-key*|change-me*|changeme*|default*|example*|test-key*|proxy-key*|management-key*) return 1 ;;
  esac
}

[ "$(id -u)" -eq 0 ] || fail
[ -d "$DATA_DIR" ] || fail
case "$(stat -c '%u:%g:%a' "$DATA_DIR")" in
  0:0:755|10001:10001:750) ;;
  *) fail ;;
esac

proxy_key=${CLIPROXY_PROXY_KEY-}
management_key=${CLIPROXY_MANAGEMENT_KEY-}
valid_key "$proxy_key" || fail
valid_key "$management_key" || fail
[ "$proxy_key" != "$management_key" ] || fail

install -d -m 0700 -o "$APP_UID" -g "$APP_GID" \
  "$DATA_DIR/auth" "$DATA_DIR/home" "$DATA_DIR/state"
chown "$APP_UID:$APP_GID" "$DATA_DIR"
chmod 0750 "$DATA_DIR"

install -d -m 0700 -o "$APP_UID" -g "$APP_GID" "$RUN_DIR"
umask 077
{
  printf '%s\n' 'host: "127.0.0.1"'
  printf '%s\n' 'port: 8317'
  printf '%s\n' 'tls:'
  printf '%s\n' '  enable: false'
  printf '%s\n' 'remote-management:'
  printf '%s\n' '  allow-remote: true'
  printf '  secret-key: "%s"\n' "$management_key"
  printf '%s\n' '  disable-control-panel: false'
  printf '%s\n' '  disable-auto-update-panel: true'
  printf '%s\n' 'auth-dir: "/data/auth"'
  printf '%s\n' 'api-keys:'
  printf '  - "%s"\n' "$proxy_key"
  printf '%s\n' 'debug: false'
  printf '%s\n' 'logging-to-file: false'
  printf '%s\n' 'usage-statistics-enabled: false'
} > "$CONFIG_FILE"
chown "$APP_UID:$APP_GID" "$CONFIG_FILE"
chmod 0600 "$CONFIG_FILE"

unset proxy_key management_key CLIPROXY_PROXY_KEY CLIPROXY_MANAGEMENT_KEY
export HOME="$DATA_DIR/home"

exec setpriv \
  --reuid="$APP_UID" \
  --regid="$APP_GID" \
  --clear-groups \
  --no-new-privs \
  /usr/local/bin/health-proxy \
    -listen "0.0.0.0:${PORT:-8080}" \
    -upstream "127.0.0.1:8317" \
    -binary "/CLIProxyAPI/CLIProxyAPI" \
    -config "$CONFIG_FILE"

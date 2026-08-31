#!/usr/bin/env python3
import json
import os
import sqlite3
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import zlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

UPSTREAM_BASE = "http://127.0.0.1:8317"
UPSTREAM = UPSTREAM_BASE + "/v1/responses"
ANTHROPIC_BASE = "http://127.0.0.1:8317"
MANAGEMENT_UPSTREAM = "http://127.0.0.1:8317/v0/management"
MANAGEMENT_PANEL = "http://127.0.0.1:8317/management.html"
MANAGEMENT_PREFIX = "/settings"
CACHE_DB_PATH = "/data/state/edge-shim-cache.sqlite3"
LEGACY_CACHE_PATH = "/data/state/edge-shim-cache.json"
CACHE_TTL_SECONDS = 24 * 60 * 60
MAX_CACHE_ENTRIES = 10_000
MAX_CACHE_ENTRY_BYTES = 64 * 1024 * 1024
MAX_LEGACY_MIGRATION_BYTES = 32 * 1024 * 1024
CACHE_LOCK = threading.RLock()

HOP_BY_HOP = {
    "connection",
    "keep-alive",
    "proxy-authenticate",
    "proxy-authorization",
    "te",
    "trailers",
    "transfer-encoding",
    "upgrade",
    "host",
    "content-length",
}


def now():
    return time.time()


def cache_connection():
    connection = sqlite3.connect(CACHE_DB_PATH, timeout=30)
    connection.execute("PRAGMA busy_timeout = 30000")
    return connection


def init_cache():
    directory = os.path.dirname(CACHE_DB_PATH)
    if directory:
        os.makedirs(directory, exist_ok=True)
    with CACHE_LOCK, cache_connection() as connection:
        connection.execute("PRAGMA journal_mode = WAL")
        connection.execute("PRAGMA synchronous = NORMAL")
        connection.execute(
            """
            CREATE TABLE IF NOT EXISTS responses (
                response_id TEXT PRIMARY KEY,
                ts REAL NOT NULL,
                raw_bytes INTEGER NOT NULL,
                payload BLOB NOT NULL
            )
            """
        )
        connection.execute("CREATE INDEX IF NOT EXISTS responses_ts ON responses(ts)")
    migrate_legacy_cache()
    prune_cache()


def migrate_legacy_cache():
    try:
        size = os.path.getsize(LEGACY_CACHE_PATH)
    except FileNotFoundError:
        return
    if size > MAX_LEGACY_MIGRATION_BYTES:
        print("legacy cache is too large to migrate safely", flush=True)
        return
    try:
        with open(LEGACY_CACHE_PATH, "r", encoding="utf-8") as handle:
            data = json.load(handle)
        if isinstance(data, dict):
            for response_id, entry in data.items():
                if isinstance(entry, dict):
                    cache_put(response_id, entry, prune=False)
        os.replace(LEGACY_CACHE_PATH, LEGACY_CACHE_PATH + ".migrated")
        print("legacy cache migrated to SQLite", flush=True)
    except Exception as exc:
        print("legacy cache migration failed: %s" % exc, flush=True)


def cache_get(response_id):
    if not response_id:
        return {}
    try:
        with CACHE_LOCK, cache_connection() as connection:
            row = connection.execute(
                "SELECT payload FROM responses WHERE response_id = ?",
                (response_id,),
            ).fetchone()
            if not row:
                return {}
            connection.execute(
                "UPDATE responses SET ts = ? WHERE response_id = ?",
                (now(), response_id),
            )
        return json.loads(zlib.decompress(row[0]).decode("utf-8"))
    except Exception as exc:
        print("cache read failed for %s: %s" % (response_id, exc), flush=True)
        return {}


def cache_put(response_id, entry, prune=True):
    if not response_id or not isinstance(entry, dict):
        return False
    try:
        raw = json.dumps(entry, separators=(",", ":")).encode("utf-8")
        if len(raw) > MAX_CACHE_ENTRY_BYTES:
            print("cache entry exceeds limit for %s" % response_id, flush=True)
            return False
        payload = zlib.compress(raw, level=3)
        timestamp = float(entry.get("ts") or now())
        with CACHE_LOCK, cache_connection() as connection:
            connection.execute(
                """
                INSERT INTO responses(response_id, ts, raw_bytes, payload)
                VALUES (?, ?, ?, ?)
                ON CONFLICT(response_id) DO UPDATE SET
                    ts = excluded.ts,
                    raw_bytes = excluded.raw_bytes,
                    payload = excluded.payload
                """,
                (response_id, timestamp, len(raw), payload),
            )
        if prune:
            prune_cache()
        return True
    except Exception as exc:
        print("cache write failed for %s: %s" % (response_id, exc), flush=True)
        return False


def prune_cache():
    try:
        cutoff = now() - CACHE_TTL_SECONDS
        with CACHE_LOCK, cache_connection() as connection:
            connection.execute("DELETE FROM responses WHERE ts < ?", (cutoff,))
            count = connection.execute("SELECT COUNT(*) FROM responses").fetchone()[0]
            excess = count - MAX_CACHE_ENTRIES
            if excess > 0:
                connection.execute(
                    """
                    DELETE FROM responses
                    WHERE response_id IN (
                        SELECT response_id FROM responses ORDER BY ts ASC LIMIT ?
                    )
                    """,
                    (excess,),
                )
    except Exception as exc:
        print("cache prune failed: %s" % exc, flush=True)


def record_response_object(response):
    if not isinstance(response, dict):
        return
    response_id = response.get("id")
    output = response.get("output") or []
    calls = {}
    for item in output:
        if not isinstance(item, dict):
            continue
        if item.get("type") in ("function_call", "custom_tool_call") and item.get("call_id"):
            calls[item["call_id"]] = item
    if response_id and calls:
        entry = cache_get(response_id)
        entry.setdefault("calls", {}).update(calls)
        entry["ts"] = now()
        cache_put(response_id, entry)


def normalize_input_items(inputs):
    if isinstance(inputs, list):
        return inputs
    if isinstance(inputs, str):
        return [{"role": "user", "content": inputs}]
    return []


def collect_text(value, parts):
    if isinstance(value, str):
        parts.append(value)
    elif isinstance(value, list):
        for item in value:
            collect_text(item, parts)
    elif isinstance(value, dict):
        for key in ("text", "content", "input", "summary"):
            if key in value:
                collect_text(value[key], parts)


def looks_like_compaction(inputs):
    parts = []
    collect_text(inputs, parts)
    text = "\n".join(parts).strip().lower()
    if not text:
        return False
    text = text[:4000]
    markers = (
        "compacted conversation summary:",
        "conversation summary:",
        "previous conversation summary:",
        "summary of the conversation:",
        "summary of previous conversation:",
        "summarized conversation:",
        "context summary:",
        "compact summary:",
    )
    return any(marker in text for marker in markers)


def cache_response_history(response, request_items):
    if not isinstance(response, dict):
        return
    response_id = response.get("id")
    if not response_id:
        return
    output = response.get("output") or []
    if not isinstance(output, list):
        output = []
    entry = cache_get(response_id)
    entry["ts"] = now()
    entry["items"] = list(request_items) + output

    calls = entry.setdefault("calls", {})
    for item in output:
        if not isinstance(item, dict):
            continue
        if item.get("type") in ("function_call", "custom_tool_call") and item.get("call_id"):
            calls[item["call_id"]] = item
    cache_put(response_id, entry)


def rewrite_payload(raw):
    try:
        payload = json.loads(raw)
    except Exception:
        return raw
    if not isinstance(payload, dict):
        return raw

    previous_id = payload.get("previous_response_id")
    inputs = payload.get("input")
    if not previous_id:
        return raw

    if looks_like_compaction(inputs):
        payload.pop("previous_response_id", None)
        return json.dumps(payload, separators=(",", ":")).encode("utf-8")

    cached_entry = cache_get(previous_id)
    cached_items = cached_entry.get("items")
    if cached_items:
        payload["input"] = list(cached_items) + normalize_input_items(inputs)
        payload.pop("previous_response_id", None)
        return json.dumps(payload, separators=(",", ":")).encode("utf-8")

    if not isinstance(inputs, list):
        return raw

    cached = cached_entry.get("calls", {})
    if not cached:
        missing_call_ids = [
            item.get("call_id")
            for item in inputs
            if isinstance(item, dict)
            and item.get("type") in ("function_call_output", "custom_tool_call_output")
            and item.get("call_id")
        ]
        if missing_call_ids:
            print(
                "cache miss for previous_response_id=%s call_ids=%s" % (previous_id, ",".join(missing_call_ids)),
                flush=True,
            )
        return raw

    rewritten = []
    inserted = set()
    changed = False
    for item in inputs:
        if isinstance(item, dict) and item.get("type") in ("function_call_output", "custom_tool_call_output"):
            call_id = item.get("call_id")
            call = cached.get(call_id)
            if call and call_id not in inserted:
                rewritten.append(call)
                inserted.add(call_id)
                changed = True
        rewritten.append(item)

    if not changed:
        return raw

    payload["input"] = rewritten
    payload.pop("previous_response_id", None)
    return json.dumps(payload, separators=(",", ":")).encode("utf-8")


def rewrite_claude_custom_tools(raw):
    try:
        payload = json.loads(raw)
    except Exception:
        return raw, set()
    if not isinstance(payload, dict) or not str(payload.get("model", "")).startswith("claude-"):
        return raw, set()

    custom_names = set()
    tools = payload.get("tools")
    if not isinstance(tools, list):
        return raw, custom_names

    for tool in tools:
        if not isinstance(tool, dict) or tool.get("type") != "custom" or not tool.get("name"):
            continue
        name = str(tool["name"])
        custom_names.add(name)
        description = str(tool.get("description") or "")
        format_definition = tool.get("format")
        if format_definition:
            description += "\n\nThe function argument `input` must contain the raw custom-tool input."
        tool.clear()
        tool.update(
            {
                "type": "function",
                "name": name,
                "description": description,
                "strict": False,
                "parameters": {
                    "type": "object",
                    "properties": {
                        "input": {
                            "type": "string",
                            "description": "Raw input for the custom tool.",
                        }
                    },
                    "required": ["input"],
                    "additionalProperties": False,
                },
            }
        )

    if not custom_names:
        return raw, custom_names

    inputs = payload.get("input")
    if isinstance(inputs, list):
        for item in inputs:
            if not isinstance(item, dict):
                continue
            if item.get("type") == "custom_tool_call" and item.get("name") in custom_names:
                item["type"] = "function_call"
                item["arguments"] = json.dumps(
                    {"input": str(item.pop("input", ""))},
                    separators=(",", ":"),
                )
            elif item.get("type") == "custom_tool_call_output":
                item["type"] = "function_call_output"

    return json.dumps(payload, separators=(",", ":")).encode("utf-8"), custom_names


def custom_tool_input(arguments):
    if not isinstance(arguments, str):
        return ""
    try:
        parsed = json.loads(arguments)
    except Exception:
        return arguments
    if isinstance(parsed, dict) and isinstance(parsed.get("input"), str):
        return parsed["input"]
    return arguments


def rewrite_custom_call_item(item, custom_names):
    if not isinstance(item, dict):
        return item
    if item.get("type") != "function_call" or item.get("name") not in custom_names:
        return item
    rewritten = dict(item)
    rewritten["type"] = "custom_tool_call"
    rewritten["input"] = custom_tool_input(rewritten.pop("arguments", ""))
    return rewritten


def rewrite_custom_response(response, custom_names):
    if not isinstance(response, dict) or not custom_names:
        return response
    rewritten = dict(response)
    output = rewritten.get("output")
    if isinstance(output, list):
        rewritten["output"] = [rewrite_custom_call_item(item, custom_names) for item in output]
    return rewritten


def rewrite_custom_stream_event(event, custom_names, custom_item_ids):
    if not isinstance(event, dict) or not custom_names:
        return event
    event_type = event.get("type")
    if event_type == "response.output_item.added":
        item = event.get("item")
        if isinstance(item, dict) and item.get("type") == "function_call" and item.get("name") in custom_names:
            item_id = item.get("id")
            if item_id:
                custom_item_ids.add(item_id)
            event = dict(event)
            event["item"] = rewrite_custom_call_item(item, custom_names)
    elif event_type == "response.function_call_arguments.delta" and event.get("item_id") in custom_item_ids:
        return None
    elif event_type == "response.function_call_arguments.done" and event.get("item_id") in custom_item_ids:
        event = dict(event)
        event["type"] = "response.custom_tool_call_input.done"
        event["input"] = custom_tool_input(event.pop("arguments", ""))
    elif event_type == "response.output_item.done":
        event = dict(event)
        event["item"] = rewrite_custom_call_item(event.get("item"), custom_names)
    elif event_type == "response.completed":
        event = dict(event)
        event["response"] = rewrite_custom_response(event.get("response"), custom_names)
    return event


class ShimHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def upstream_target(self):
        parsed = urllib.parse.urlsplit(self.path)
        target = UPSTREAM_BASE + parsed.path
        if parsed.query:
            target += "?" + parsed.query
        return target

    def management_target(self):
        parsed = urllib.parse.urlsplit(self.path)
        path = parsed.path
        if path != MANAGEMENT_PREFIX and not path.startswith(MANAGEMENT_PREFIX + "/"):
            return None
        suffix = path[len(MANAGEMENT_PREFIX) :]
        target = MANAGEMENT_UPSTREAM + suffix
        if parsed.query:
            target += "?" + parsed.query
        return target

    def anthropic_target(self):
        parsed = urllib.parse.urlsplit(self.path)
        path = parsed.path
        if path != "/v1/messages" and not path.startswith("/v1/messages/"):
            return None
        target = ANTHROPIC_BASE + path
        if parsed.query:
            target += "?" + parsed.query
        return target

    def is_management_panel_path(self):
        parsed = urllib.parse.urlsplit(self.path)
        return parsed.path in (MANAGEMENT_PREFIX, MANAGEMENT_PREFIX + "/")

    def do_GET(self):
        if self.is_management_panel_path():
            self.forward_management_panel()
            return
        target = self.management_target()
        if target:
            self.forward_management(target, b"")
            return
        if self.path in ("/health", "/healthz"):
            body = b"ok\n"
            self.send_response(200)
            self.send_header("content-type", "text/plain")
            self.send_header("content-length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self.forward_management(self.upstream_target(), b"")

    def do_PUT(self):
        self.forward_management_method()

    def do_PATCH(self):
        self.forward_management_method()

    def do_DELETE(self):
        self.forward_management_method()

    def do_OPTIONS(self):
        target = self.management_target()
        if target:
            self.forward_management(target, b"")
            return
        self.forward_management(self.upstream_target(), b"")

    def forward_management_method(self):
        target = self.management_target()
        target = target or self.upstream_target()
        length = int(self.headers.get("content-length") or "0")
        raw = self.rfile.read(length)
        self.forward_management(target, raw)

    def forward_management(self, target, body):
        headers = {}
        for key, value in self.headers.items():
            if key.lower() not in HOP_BY_HOP:
                headers[key] = value
        if body:
            headers["Content-Length"] = str(len(body))
        req = urllib.request.Request(target, data=body if body else None, headers=headers, method=self.command)
        try:
            resp = urllib.request.urlopen(req, timeout=900)
            self.forward_plain_response(resp)
        except urllib.error.HTTPError as exc:
            self.forward_plain_response(exc)
        except Exception as exc:
            payload = json.dumps({"error": {"message": str(exc), "type": "shim_error"}}).encode()
            self.send_response(502)
            self.send_header("content-type", "application/json")
            self.send_header("content-length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

    def forward_management_panel(self):
        headers = {}
        for key, value in self.headers.items():
            if key.lower() not in HOP_BY_HOP:
                headers[key] = value
        req = urllib.request.Request(MANAGEMENT_PANEL, headers=headers, method="GET")
        try:
            resp = urllib.request.urlopen(req, timeout=900)
            status = getattr(resp, "status", None) or resp.code
            body = resp.read().replace(b"`/v0/management`", b"`/settings`")
            self.send_response(status)
            for key, value in resp.headers.items():
                lower = key.lower()
                if lower not in HOP_BY_HOP and lower not in ("content-length", "content-encoding"):
                    self.send_header(key, value)
            self.send_header("content-length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        except urllib.error.HTTPError as exc:
            self.forward_plain_response(exc)
        except Exception as exc:
            payload = json.dumps({"error": {"message": str(exc), "type": "shim_error"}}).encode()
            self.send_response(502)
            self.send_header("content-type", "application/json")
            self.send_header("content-length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

    def do_POST(self):
        target = self.management_target()
        if target:
            length = int(self.headers.get("content-length") or "0")
            raw = self.rfile.read(length)
            self.forward_management(target, raw)
            return

        length = int(self.headers.get("content-length") or "0")
        raw = self.rfile.read(length)
        parsed = urllib.parse.urlsplit(self.path)
        is_responses = parsed.path == "/v1/responses"
        target = self.anthropic_target()
        rewritten = rewrite_payload(raw) if is_responses else raw
        custom_names = set()
        if is_responses:
            rewritten, custom_names = rewrite_claude_custom_tools(rewritten)
        target = target or (UPSTREAM if is_responses else self.upstream_target())
        request_items = []
        if target == UPSTREAM:
            try:
                request_items = normalize_input_items(json.loads(rewritten).get("input")) if rewritten else []
            except Exception:
                pass

        headers = {}
        for key, value in self.headers.items():
            if key.lower() not in HOP_BY_HOP:
                headers[key] = value
        headers["Content-Length"] = str(len(rewritten))

        req = urllib.request.Request(target, data=rewritten, headers=headers, method="POST")
        try:
            resp = urllib.request.urlopen(req, timeout=900)
            self.forward_response(resp, request_items, custom_names)
        except urllib.error.HTTPError as exc:
            self.forward_response(exc, request_items, custom_names)
        except Exception as exc:
            body = json.dumps({"error": {"message": str(exc), "type": "shim_error"}}).encode()
            self.send_response(502)
            self.send_header("content-type", "application/json")
            self.send_header("content-length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    def forward_plain_response(self, resp):
        status = getattr(resp, "status", None) or resp.code
        body = resp.read()
        self.send_response(status)
        for key, value in resp.headers.items():
            if key.lower() not in HOP_BY_HOP:
                self.send_header(key, value)
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def forward_response(self, resp, request_items, custom_names):
        status = getattr(resp, "status", None) or resp.code
        content_type = resp.headers.get("content-type", "")
        self.send_response(status)
        for key, value in resp.headers.items():
            if key.lower() not in HOP_BY_HOP and key.lower() != "content-length":
                self.send_header(key, value)

        if "text/event-stream" in content_type.lower():
            self.send_header("connection", "close")
            self.end_headers()
            self.forward_stream(resp, request_items, custom_names)
            return

        body = resp.read()
        try:
            response = rewrite_custom_response(json.loads(body), custom_names)
            body = json.dumps(response, separators=(",", ":")).encode("utf-8")
            record_response_object(response)
            cache_response_history(response, request_items)
        except Exception:
            pass
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def forward_stream(self, resp, request_items, custom_names):
        current_response_id = None
        calls = {}
        final_response = None
        custom_item_ids = set()
        for raw_line in resp:
            line = raw_line.decode("utf-8", "replace").strip()
            if not line.startswith("data: "):
                self.wfile.write(raw_line)
                self.wfile.flush()
                continue
            data = line[6:]
            if not data or data == "[DONE]":
                self.wfile.write(raw_line)
                self.wfile.flush()
                continue
            try:
                event = json.loads(data)
            except Exception:
                self.wfile.write(raw_line)
                self.wfile.flush()
                continue
            event = rewrite_custom_stream_event(event, custom_names, custom_item_ids)
            if event is None:
                continue
            raw_line = ("data: " + json.dumps(event, separators=(",", ":")) + "\n").encode("utf-8")
            self.wfile.write(raw_line)
            self.wfile.flush()
            event_type = event.get("type")
            if event_type == "response.created":
                response = event.get("response") or {}
                current_response_id = response.get("id") or current_response_id
            elif event_type == "response.output_item.done":
                item = event.get("item") or {}
                if item.get("type") in ("function_call", "custom_tool_call") and item.get("call_id"):
                    calls[item["call_id"]] = item
            elif event_type == "response.completed":
                response = event.get("response") or {}
                final_response = response
                current_response_id = response.get("id") or current_response_id
                record_response_object(response)
                cache_response_history(response, request_items)
        if current_response_id and calls:
            entry = cache_get(current_response_id)
            entry["ts"] = now()
            entry.setdefault("calls", {}).update(calls)
            if final_response is not None and "items" not in entry:
                output = final_response.get("output") or []
                entry["items"] = list(request_items) + (output if isinstance(output, list) else [])
            cache_put(current_response_id, entry)

    def log_message(self, fmt, *args):
        print("%s - %s" % (self.address_string(), fmt % args), flush=True)


if __name__ == "__main__":
    init_cache()
    server = ThreadingHTTPServer(("0.0.0.0", 8320), ShimHandler)
    print("edge shim listening on :8320", flush=True)
    server.serve_forever()

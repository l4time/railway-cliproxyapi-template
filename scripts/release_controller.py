#!/usr/bin/env python3
"""Fail-closed CLIProxyAPI release promotion and rollback controller.

The workflow supplies metadata from the GitHub Releases API and an immutable
Docker registry manifest digest. This program validates policy, atomically
updates both Dockerfile pins plus release-state.json, and never reads Railway
credentials or application secrets.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import tempfile
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any


STABLE_TAG = re.compile(r"^v[0-9]+\.[0-9]+\.[0-9]+$")
DIGEST = re.compile(r"^sha256:[0-9a-f]{64}$")
PIN = re.compile(
    r"^ARG UPSTREAM_IMAGE=eceasy/cli-proxy-api@sha256:[0-9a-f]{64}$",
    re.MULTILINE,
)
SOURCE_FILES = ("Dockerfile", "entrypoint.sh", "health-proxy.go")
SOURCE_LINE = re.compile(
    r"^(?P<digest>[0-9a-f]{64})  "
    r"(?P<name>Dockerfile|entrypoint\.sh|health-proxy\.go)$"
)


def parse_time(value: str) -> datetime:
    parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        raise ValueError("timestamp must include a timezone")
    return parsed.astimezone(timezone.utc)


def format_time(value: datetime) -> str:
    return value.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")


def semver(tag: str) -> tuple[int, int, int]:
    if not STABLE_TAG.fullmatch(tag):
        raise ValueError(f"invalid stable tag: {tag}")
    return tuple(int(part) for part in tag[1:].split("."))  # type: ignore[return-value]


@dataclass(frozen=True)
class Candidate:
    tag: str
    digest: str
    published_at: datetime
    prerelease: bool
    draft: bool


def decision(candidate: Candidate, state: dict[str, Any], now: datetime) -> str:
    if candidate.draft:
        return "reject:draft"
    if candidate.prerelease or not STABLE_TAG.fullmatch(candidate.tag):
        return "reject:unstable-tag"
    if not DIGEST.fullmatch(candidate.digest):
        return "reject:digest"
    candidate_version = semver(candidate.tag)
    current_version = semver(state["current"]["tag"])
    if candidate_version < current_version:
        return "reject:non-forward"
    if candidate_version == current_version:
        if candidate.digest == state["current"]["digest"]:
            return "noop:current"
        return "reject:current-tag-digest-drift"
    if candidate.published_at > now:
        return "reject:future-release"
    if now - candidate.published_at < timedelta(hours=12):
        return "defer:soak"
    if candidate.digest == state["current"]["digest"]:
        return "noop:current-digest"
    if candidate.digest == state.get("prior", {}).get("digest"):
        return "reject:known-rollback"
    last = parse_time(state["current"]["promotedAt"])
    if now - last < timedelta(hours=24):
        return "defer:daily-limit"
    return "promote"


def smoke_targets(state: dict[str, Any], mode: str) -> list[str]:
    """Return the only image digests a workflow mode is allowed to boot."""
    if mode == "rollback-target":
        return [state["current"]["digest"]]
    if mode == "full":
        return [
            state["prior"]["digest"],
            state["current"]["digest"],
            state["prior"]["digest"],
            state["current"]["digest"],
        ]
    raise ValueError(f"unsupported smoke mode: {mode}")


def commit_allowed(smoke_results: list[bool]) -> bool:
    """A changed release may commit only after every required target passes."""
    return bool(smoke_results) and all(smoke_results)


def load_state(path: Path) -> dict[str, Any]:
    state = json.loads(path.read_text(encoding="utf-8"))
    if state.get("schema") != 1:
        raise ValueError("unsupported release-state schema")
    if state.get("upstreamRepository") != "router-for-me/CLIProxyAPI":
        raise ValueError("unexpected upstream repository")
    if state.get("imageRepository") != "eceasy/cli-proxy-api":
        raise ValueError("unexpected image repository")
    for slot in ("current", "prior"):
        record = state.get(slot)
        if not isinstance(record, dict):
            raise ValueError(f"missing {slot} release record")
        if not STABLE_TAG.fullmatch(str(record.get("tag", ""))):
            raise ValueError(f"invalid {slot} tag")
        if not DIGEST.fullmatch(str(record.get("digest", ""))):
            raise ValueError(f"invalid {slot} digest")
    return state


def render_pin(source: str, digest: str) -> str:
    replacement = f"ARG UPSTREAM_IMAGE=eceasy/cli-proxy-api@{digest}"
    updated, count = PIN.subn(replacement, source)
    if count != 2:
        raise ValueError(f"expected exactly two upstream pins, found {count}")
    return updated


def write_pin(dockerfile: Path, digest: str) -> None:
    dockerfile.write_text(
        render_pin(dockerfile.read_text(encoding="utf-8"), digest),
        encoding="utf-8",
    )


def parse_source_sums(source: str) -> dict[str, str]:
    lines = source.splitlines()
    if len(lines) != len(SOURCE_FILES):
        raise ValueError("SOURCE_SHA256SUMS must contain exactly three records")
    parsed: dict[str, str] = {}
    for line in lines:
        match = SOURCE_LINE.fullmatch(line)
        if not match:
            raise ValueError("malformed SOURCE_SHA256SUMS record")
        name = match.group("name")
        if name in parsed:
            raise ValueError(f"duplicate SOURCE_SHA256SUMS record: {name}")
        parsed[name] = match.group("digest")
    if tuple(parsed) != SOURCE_FILES:
        raise ValueError("SOURCE_SHA256SUMS records are missing or out of order")
    return parsed


def render_source_sums(
    source: str, current_dockerfile: bytes, updated_dockerfile: bytes
) -> str:
    parsed = parse_source_sums(source)
    current_hash = hashlib.sha256(current_dockerfile).hexdigest()
    if parsed["Dockerfile"] != current_hash:
        raise ValueError("stale Dockerfile SOURCE_SHA256SUMS record")
    parsed["Dockerfile"] = hashlib.sha256(updated_dockerfile).hexdigest()
    return "".join(f"{parsed[name]}  {name}\n" for name in SOURCE_FILES)


def verify_source_checksum(source_sums: Path, dockerfile: Path) -> None:
    parsed = parse_source_sums(source_sums.read_text(encoding="utf-8"))
    actual = hashlib.sha256(dockerfile.read_bytes()).hexdigest()
    if parsed["Dockerfile"] != actual:
        raise ValueError("Dockerfile checksum verification failed")


def atomic_write(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    handle, temporary = tempfile.mkstemp(
        prefix=f".{path.name}.", suffix=".tmp", dir=path.parent
    )
    try:
        with os.fdopen(handle, "w", encoding="utf-8") as stream:
            stream.write(content)
            stream.flush()
            os.fsync(stream.fileno())
        if path.exists():
            os.chmod(temporary, path.stat().st_mode & 0o777)
        os.replace(temporary, path)
    except BaseException:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def write_release_files(
    dockerfile: Path,
    state_path: Path,
    source_sums: Path,
    state: dict[str, Any],
    digest: str,
) -> None:
    originals = {
        dockerfile: dockerfile.read_text(encoding="utf-8"),
        state_path: state_path.read_text(encoding="utf-8"),
        source_sums: source_sums.read_text(encoding="utf-8"),
    }
    updated_dockerfile = render_pin(originals[dockerfile], digest)
    updated_sums = render_source_sums(
        originals[source_sums],
        originals[dockerfile].encode(),
        updated_dockerfile.encode(),
    )
    updates = {
        dockerfile: updated_dockerfile,
        state_path: json.dumps(state, indent=2) + "\n",
        source_sums: updated_sums,
    }
    written: list[Path] = []
    try:
        for path in (dockerfile, state_path, source_sums):
            atomic_write(path, updates[path])
            written.append(path)
        verify_source_checksum(source_sums, dockerfile)
    except BaseException:
        for path in reversed(written):
            atomic_write(path, originals[path])
        raise


def candidate_from_args(args: argparse.Namespace) -> Candidate:
    return Candidate(
        tag=args.tag,
        digest=args.digest,
        published_at=parse_time(args.published_at),
        prerelease=args.prerelease,
        draft=args.draft,
    )


def print_result(result: str, **extra: Any) -> None:
    print(json.dumps({"decision": result, **extra}, sort_keys=True))


def command_check(args: argparse.Namespace) -> int:
    state = load_state(args.state)
    candidate = candidate_from_args(args)
    result = decision(candidate, state, parse_time(args.now))
    print_result(result, tag=candidate.tag, digest=candidate.digest)
    return 0


def command_promote(args: argparse.Namespace) -> int:
    state = load_state(args.state)
    candidate = candidate_from_args(args)
    now = parse_time(args.now)
    result = decision(candidate, state, now)
    if result != "promote":
        print_result(result)
        return 3
    old = state["current"]
    state["prior"] = old
    state["current"] = {
        "tag": candidate.tag,
        "digest": candidate.digest,
        "publishedAt": format_time(candidate.published_at),
        "promotedAt": format_time(now),
    }
    write_release_files(
        args.dockerfile, args.state, args.source_sums, state, candidate.digest
    )
    print_result("promote", current=state["current"], prior=state["prior"])
    return 0


def command_rollback(args: argparse.Namespace) -> int:
    state = load_state(args.state)
    now = parse_time(args.now)
    current, prior = state["current"], state["prior"]
    state["current"] = {
        **prior,
        "promotedAt": format_time(now),
    }
    state["prior"] = current
    write_release_files(
        args.dockerfile,
        args.state,
        args.source_sums,
        state,
        state["current"]["digest"],
    )
    print_result("rollback", current=state["current"], prior=state["prior"])
    return 0


def add_candidate_args(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--tag", required=True)
    parser.add_argument("--digest", required=True)
    parser.add_argument("--published-at", required=True)
    parser.add_argument("--prerelease", action="store_true")
    parser.add_argument("--draft", action="store_true")
    parser.add_argument("--now", required=True)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--state", type=Path, default=Path("release-state.json"))
    parser.add_argument("--dockerfile", type=Path, default=Path("Dockerfile"))
    parser.add_argument(
        "--source-sums", type=Path, default=Path("SOURCE_SHA256SUMS")
    )
    sub = parser.add_subparsers(dest="command", required=True)
    check = sub.add_parser("check")
    add_candidate_args(check)
    check.set_defaults(handler=command_check)
    promote = sub.add_parser("promote")
    add_candidate_args(promote)
    promote.set_defaults(handler=command_promote)
    rollback = sub.add_parser("rollback")
    rollback.add_argument("--now", required=True)
    rollback.set_defaults(handler=command_rollback)
    args = parser.parse_args()
    try:
        return args.handler(args)
    except (OSError, ValueError, KeyError, json.JSONDecodeError) as exc:
        print(f"release controller failed closed: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())

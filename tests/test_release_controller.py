#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import hashlib
import json
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "release_controller", ROOT / "scripts" / "release_controller.py"
)
assert SPEC and SPEC.loader
controller = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = controller
SPEC.loader.exec_module(controller)


def digest(char: str) -> str:
    return "sha256:" + char * 64


def write_source_sums(path: Path, dockerfile: Path) -> None:
    path.write_text(
        f"{hashlib.sha256(dockerfile.read_bytes()).hexdigest()}  Dockerfile\n"
        f"{'a' * 64}  entrypoint.sh\n"
        f"{'b' * 64}  config-reconciler.go\n"
        f"{'c' * 64}  health-proxy.go\n"
        f"{'d' * 64}  health-proxy_test.go\n",
        encoding="utf-8",
    )


class ReleaseControllerTests(unittest.TestCase):
    def setUp(self) -> None:
        self.now = datetime(2026, 8, 25, 18, tzinfo=timezone.utc)
        self.state = {
            "schema": 1,
            "upstreamRepository": "router-for-me/CLIProxyAPI",
            "imageRepository": "eceasy/cli-proxy-api",
            "current": {
                "tag": "v1.2.3",
                "digest": digest("1"),
                "publishedAt": "2026-08-20T00:00:00Z",
                "promotedAt": "2026-08-25T00:00:00Z",
            },
            "prior": {
                "tag": "v1.2.2",
                "digest": digest("0"),
                "publishedAt": "2026-08-19T00:00:00Z",
                "promotedAt": "2026-08-23T00:00:00Z",
            },
        }

    def candidate(self, **changes: object):
        values = {
            "tag": "v1.2.4",
            "digest": digest("2"),
            "published_at": self.now - timedelta(hours=13),
            "prerelease": False,
            "draft": False,
        }
        values.update(changes)
        return controller.Candidate(**values)

    def test_policy_matrix(self) -> None:
        cases = {
            "promote": self.candidate(),
            "reject:draft": self.candidate(draft=True),
            "reject:unstable-tag": self.candidate(tag="v1.2.4-rc1"),
            "reject:digest": self.candidate(digest="latest"),
            "reject:future-release": self.candidate(
                published_at=self.now + timedelta(minutes=1)
            ),
            "defer:soak": self.candidate(
                published_at=self.now - timedelta(hours=11)
            ),
            "noop:current-digest": self.candidate(
                digest=self.state["current"]["digest"]
            ),
            "noop:current": self.candidate(
                tag=self.state["current"]["tag"],
                digest=self.state["current"]["digest"],
            ),
            "reject:current-tag-digest-drift": self.candidate(
                tag=self.state["current"]["tag"],
                digest=digest("9"),
            ),
            "reject:non-forward": self.candidate(tag="v1.2.1"),
            "reject:known-rollback": self.candidate(
                digest=self.state["prior"]["digest"]
            ),
            "defer:daily-limit": self.candidate(),
        }
        for expected, candidate in cases.items():
            state = json.loads(json.dumps(self.state))
            if expected == "promote":
                state["current"]["promotedAt"] = "2026-08-24T10:00:00Z"
            with self.subTest(expected=expected):
                self.assertEqual(
                    controller.decision(candidate, state, self.now), expected
                )

    def test_numeric_semver_ordering_and_explicit_older_rejection(self) -> None:
        state = json.loads(json.dumps(self.state))
        state["current"]["tag"] = "v7.2.141"
        state["current"]["promotedAt"] = "2026-08-24T10:00:00Z"
        older = self.candidate(tag="v7.2.139", digest=digest("8"))
        self.assertEqual(
            controller.decision(older, state, self.now), "reject:non-forward"
        )

        state["current"]["tag"] = "v7.9.99"
        newer = self.candidate(tag="v7.10.0", digest=digest("8"))
        self.assertEqual(controller.decision(newer, state, self.now), "promote")
        self.assertGreater(controller.semver("v12.0.0"), controller.semver("v9.99.99"))
        self.assertGreater(controller.semver("v7.2.10"), controller.semver("v7.2.9"))

    def test_write_pin_requires_exactly_two_locked_lines(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            dockerfile = Path(temp) / "Dockerfile"
            old = digest("1")
            dockerfile.write_text(
                f"ARG UPSTREAM_IMAGE=eceasy/cli-proxy-api@{old}\n"
                "ARG EMBEDDED_VERSION=v1.2.3\n"
                "FROM scratch\n"
                f"ARG UPSTREAM_IMAGE=eceasy/cli-proxy-api@{old}\n",
                encoding="utf-8",
            )
            controller.write_pin(dockerfile, digest("2"), "v1.2.4")
            self.assertEqual(dockerfile.read_text().count(digest("2")), 2)
            self.assertIn("ARG EMBEDDED_VERSION=v1.2.4", dockerfile.read_text())

    def test_rollback_swaps_current_and_prior(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            state_path = root / "release-state.json"
            dockerfile = root / "Dockerfile"
            source_sums = root / "SOURCE_SHA256SUMS"
            state_path.write_text(json.dumps(self.state), encoding="utf-8")
            current = self.state["current"]["digest"]
            dockerfile.write_text(
                f"ARG UPSTREAM_IMAGE=eceasy/cli-proxy-api@{current}\n"
                "ARG EMBEDDED_VERSION=v1.2.3\n"
                "FROM scratch\n"
                f"ARG UPSTREAM_IMAGE=eceasy/cli-proxy-api@{current}\n",
                encoding="utf-8",
            )
            write_source_sums(source_sums, dockerfile)
            args = type(
                "Args",
                (),
                {
                    "state": state_path,
                    "dockerfile": dockerfile,
                    "source_sums": source_sums,
                    "now": "2026-08-25T18:00:00Z",
                },
            )
            self.assertEqual(controller.command_rollback(args), 0)
            result = json.loads(state_path.read_text())
            self.assertEqual(result["current"]["tag"], "v1.2.2")
            self.assertEqual(result["prior"]["tag"], "v1.2.3")
            self.assertEqual(dockerfile.read_text().count(digest("0")), 2)
            self.assertIn("ARG EMBEDDED_VERSION=v1.2.2", dockerfile.read_text())
            controller.verify_source_checksum(source_sums, dockerfile)

    def test_promote_refreshes_dockerfile_source_checksum(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            state_path = root / "release-state.json"
            dockerfile = root / "Dockerfile"
            source_sums = root / "SOURCE_SHA256SUMS"
            state = json.loads(json.dumps(self.state))
            state["current"]["promotedAt"] = "2026-08-24T10:00:00Z"
            state_path.write_text(json.dumps(state), encoding="utf-8")
            dockerfile.write_text(
                f"ARG UPSTREAM_IMAGE=eceasy/cli-proxy-api@{digest('1')}\n"
                "ARG EMBEDDED_VERSION=v1.2.3\n"
                "FROM scratch\n"
                f"ARG UPSTREAM_IMAGE=eceasy/cli-proxy-api@{digest('1')}\n",
                encoding="utf-8",
            )
            write_source_sums(source_sums, dockerfile)
            args = type(
                "Args",
                (),
                {
                    "state": state_path,
                    "dockerfile": dockerfile,
                    "source_sums": source_sums,
                    "tag": "v1.2.4",
                    "digest": digest("2"),
                    "published_at": "2026-08-25T04:00:00Z",
                    "prerelease": False,
                    "draft": False,
                    "now": "2026-08-25T18:00:00Z",
                },
            )
            self.assertEqual(controller.command_promote(args), 0)
            controller.verify_source_checksum(source_sums, dockerfile)
            self.assertEqual(
                controller.parse_source_sums(source_sums.read_text())["Dockerfile"],
                hashlib.sha256(dockerfile.read_bytes()).hexdigest(),
            )

    def test_missing_or_malformed_source_checksum_fails_without_changes(self) -> None:
        malformed_sources = (
            f"{'a' * 64} entrypoint.sh\n{'b' * 64}  config-reconciler.go\n"
            f"{'c' * 64}  health-proxy.go\n",
            f"{'a' * 64}  entrypoint.sh\n{'b' * 64}  config-reconciler.go\n"
            f"{'c' * 64}  health-proxy.go\n",
        )
        for malformed in malformed_sources:
            with self.subTest(source=malformed), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                state_path = root / "release-state.json"
                dockerfile = root / "Dockerfile"
                source_sums = root / "SOURCE_SHA256SUMS"
                state_path.write_text(json.dumps(self.state), encoding="utf-8")
                dockerfile.write_text(
                    f"ARG UPSTREAM_IMAGE=eceasy/cli-proxy-api@{digest('1')}\n"
                    "ARG EMBEDDED_VERSION=v1.2.3\n"
                    "FROM scratch\n"
                    f"ARG UPSTREAM_IMAGE=eceasy/cli-proxy-api@{digest('1')}\n",
                    encoding="utf-8",
                )
                source_sums.write_text(malformed, encoding="utf-8")
                originals = (
                    dockerfile.read_bytes(),
                    state_path.read_bytes(),
                    source_sums.read_bytes(),
                )
                args = type(
                    "Args",
                    (),
                    {
                        "state": state_path,
                        "dockerfile": dockerfile,
                        "source_sums": source_sums,
                        "now": "2026-08-25T18:00:00Z",
                    },
                )
                with self.assertRaises(ValueError):
                    controller.command_rollback(args)
                self.assertEqual(
                    originals,
                    (
                        dockerfile.read_bytes(),
                        state_path.read_bytes(),
                        source_sums.read_bytes(),
                    ),
                )

    def test_rollback_smokes_only_target_and_failed_target_blocks_commit(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            state_path = root / "release-state.json"
            dockerfile = root / "Dockerfile"
            source_sums = root / "SOURCE_SHA256SUMS"
            state = json.loads(json.dumps(self.state))
            state["current"] = {
                **state["current"],
                "tag": "v7.2.141",
                "digest": digest("f"),  # Simulated broken outgoing image.
            }
            state["prior"] = {
                **state["prior"],
                "tag": "v7.2.140",
                "digest": digest("a"),  # Retained rollback target.
            }
            state_path.write_text(json.dumps(state), encoding="utf-8")
            dockerfile.write_text(
                f"ARG UPSTREAM_IMAGE=eceasy/cli-proxy-api@{digest('f')}\n"
                "ARG EMBEDDED_VERSION=v7.2.141\n"
                "FROM scratch\n"
                f"ARG UPSTREAM_IMAGE=eceasy/cli-proxy-api@{digest('f')}\n",
                encoding="utf-8",
            )
            write_source_sums(source_sums, dockerfile)
            args = type(
                "Args",
                (),
                {
                    "state": state_path,
                    "dockerfile": dockerfile,
                    "source_sums": source_sums,
                    "now": "2026-08-25T18:00:00Z",
                },
            )
            self.assertEqual(controller.command_rollback(args), 0)
            swapped = json.loads(state_path.read_text())
            targets = controller.smoke_targets(swapped, "rollback-target")
            self.assertEqual(targets, [digest("a")])
            self.assertNotIn(digest("f"), targets)
            self.assertTrue(controller.commit_allowed([True]))
            self.assertFalse(controller.commit_allowed([False]))
            self.assertFalse(controller.commit_allowed([]))
            controller.verify_source_checksum(source_sums, dockerfile)


if __name__ == "__main__":
    unittest.main()

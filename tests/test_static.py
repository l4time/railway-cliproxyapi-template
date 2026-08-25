#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import json
import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class StaticContractTests(unittest.TestCase):
    def test_proven_runtime_sources_are_unchanged(self) -> None:
        expected = {
            "entrypoint.sh": "39b3bd5d7e262f490172282d207a7563601d566ae52c8ba1064bbecf0fab0c77",
            "health-proxy.go": "dbc811f7b49110392ca3db0870a53fb939c0b51e86be576721a18549fb935c32",
        }
        for name, digest in expected.items():
            actual = hashlib.sha256((ROOT / name).read_bytes()).hexdigest()
            self.assertEqual(actual, digest, name)

    def test_dockerfile_matches_state_and_pinned_panel(self) -> None:
        state = json.loads((ROOT / "release-state.json").read_text())
        dockerfile = (ROOT / "Dockerfile").read_text()
        pin = f"eceasy/cli-proxy-api@{state['current']['digest']}"
        self.assertEqual(dockerfile.count(pin), 2)
        self.assertIn(
            "sha256:e2643e0875e0024e5ff9ddf4569e4c58611ab0456aeb6fa6065ed3e6c2b721f4",
            dockerfile,
        )
        self.assertIn(
            "releases/download/v1.22.6/management.html", dockerfile
        )
        self.assertNotIn(":latest", dockerfile)
        self.assertIn(
            "golang:1.25.5-bookworm@sha256:"
            "d9132cce84391efab786495288756d60e1da215b1f94e87860aeefc3d4c45b6d",
            dockerfile,
        )

    def test_railway_contract(self) -> None:
        config = json.loads((ROOT / "railway.json").read_text())
        self.assertEqual(config["build"]["builder"], "DOCKERFILE")
        deploy = config["deploy"]
        self.assertEqual(deploy["healthcheckPath"], "/healthz")
        self.assertEqual(deploy["numReplicas"], 1)
        self.assertEqual(deploy["restartPolicyType"], "ON_FAILURE")
        self.assertEqual(deploy["restartPolicyMaxRetries"], 10)
        self.assertFalse(deploy["sleepApplication"])

    def test_docs_have_no_forbidden_product_claim(self) -> None:
        text = "\n".join(
            path.read_text(errors="replace")
            for path in ROOT.rglob("*.md")
        )
        self.assertNotRegex(text.lower(), r"\bno api key needed\b")
        self.assertNotRegex(text.lower(), r"\bfully automatic railway updates\b")

    def test_no_secret_literals(self) -> None:
        suspicious = re.compile(
            r"(?i)(ghp_[a-z0-9]{20,}|github_pat_[a-z0-9_]{20,}|"
            r"railway[_-]?(?:api[_-]?)?token\s*[:=]\s*['\"][^$<])"
        )
        for path in ROOT.rglob("*"):
            if not path.is_file() or ".git" in path.parts:
                continue
            self.assertIsNone(
                suspicious.search(path.read_text(errors="replace")), str(path)
            )

    def test_workflow_isolates_rollback_target_and_gates_commit_on_success(self) -> None:
        workflow = (
            ROOT / ".github" / "workflows" / "release-controller.yml"
        ).read_text()
        self.assertIn("smoke_mode=rollback-target", workflow)
        self.assertIn(
            'tests/run.sh "${{ steps.prepare.outputs.smoke_mode }}"', workflow
        )
        self.assertIn(
            "if: success() && steps.prepare.outputs.changed == 'true'", workflow
        )
        self.assertIn(
            "git add Dockerfile release-state.json SOURCE_SHA256SUMS", workflow
        )

    def test_source_checksum_matches_current_dockerfile(self) -> None:
        controller_path = ROOT / "scripts" / "release_controller.py"
        self.assertTrue(controller_path.is_file())
        source = (ROOT / "SOURCE_SHA256SUMS").read_text()
        match = re.search(r"^([0-9a-f]{64})  Dockerfile$", source, re.MULTILINE)
        self.assertIsNotNone(match)
        assert match
        self.assertEqual(
            match.group(1), hashlib.sha256((ROOT / "Dockerfile").read_bytes()).hexdigest()
        )


if __name__ == "__main__":
    unittest.main()

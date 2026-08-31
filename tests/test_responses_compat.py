import importlib.util
import json
import os
import tempfile
import unittest


ROOT = os.path.dirname(os.path.dirname(__file__))
SPEC = importlib.util.spec_from_file_location(
    "responses_compat",
    os.path.join(ROOT, "edge-shim.py"),
)
COMPAT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(COMPAT)


class ResponsesCompatibilityTests(unittest.TestCase):
    def setUp(self):
        self.tempdir = tempfile.TemporaryDirectory()
        COMPAT.CACHE_DB_PATH = os.path.join(self.tempdir.name, "responses.sqlite3")
        COMPAT.LEGACY_CACHE_PATH = os.path.join(self.tempdir.name, "legacy.json")
        COMPAT.init_cache()

    def tearDown(self):
        self.tempdir.cleanup()

    def test_reconstructs_tool_call_before_output(self):
        call = {
            "type": "function_call",
            "id": "fc_1",
            "call_id": "call_1",
            "name": "probe",
            "arguments": "{\"value\":\"ok\"}",
        }
        COMPAT.cache_response_history(
            {"id": "resp_1", "output": [call]},
            [{"role": "user", "content": "Run the probe."}],
        )
        raw = json.dumps({
            "model": "gpt-5.6-sol",
            "previous_response_id": "resp_1",
            "input": [{
                "type": "function_call_output",
                "call_id": "call_1",
                "output": "ok",
            }],
            "store": False,
        }).encode()

        rewritten = json.loads(COMPAT.rewrite_payload(raw))

        self.assertNotIn("previous_response_id", rewritten)
        self.assertEqual(
            [item.get("type") for item in rewritten["input"]],
            [None, "function_call", "function_call_output"],
        )
        self.assertEqual(rewritten["input"][1]["call_id"], "call_1")

    def test_compaction_summary_starts_a_new_chain(self):
        raw = json.dumps({
            "model": "gpt-5.6-sol",
            "previous_response_id": "resp_old",
            "input": "Compacted conversation summary: prior work completed.",
            "store": False,
        }).encode()

        rewritten = json.loads(COMPAT.rewrite_payload(raw))

        self.assertNotIn("previous_response_id", rewritten)
        self.assertIn("Compacted conversation summary", rewritten["input"])


if __name__ == "__main__":
    unittest.main()

import unittest

from quarantine_observation import matches


class QuarantineObservationTests(unittest.TestCase):
    def test_previous_failure_cannot_complete_new_candidate(self):
        old = "v8.0.19@" + "a" * 64
        current = "v8.0.20@" + "b" * 64
        ledger = {
            "phase": "idle",
            "last_failure_class": "deterministic",
            "quarantine": {old: "release archive contract mismatch"},
        }
        self.assertFalse(matches(ledger, "deterministic", current))
        ledger["phase"] = "probe"
        ledger["quarantine"][current] = "candidate private probe failed"
        self.assertFalse(matches(ledger, "deterministic", current))
        ledger["phase"] = "idle"
        self.assertTrue(matches(ledger, "deterministic", current))

    def test_same_tag_wrong_checksum_or_class_does_not_match(self):
        identity = "v8.0.20@" + "b" * 64
        ledger = {
            "phase": "idle",
            "last_failure_class": "deterministic",
            "quarantine": {identity: "candidate private probe failed"},
        }
        self.assertFalse(matches(ledger, "security", identity))
        self.assertFalse(matches(ledger, "deterministic", "v8.0.20@" + "c" * 64))


if __name__ == "__main__":
    unittest.main()

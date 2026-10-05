"""Match a completed failure for one exact fixture candidate identity."""

import json
import sys


def matches(ledger, failure_class, identity):
    return (
        ledger.get("phase") == "idle"
        and ledger.get("last_failure_class") == failure_class
        and bool(ledger.get("quarantine", {}).get(identity))
    )


if __name__ == "__main__":
    try:
        ledger = json.load(sys.stdin)
        observed = matches(ledger, sys.argv[1], sys.argv[2])
    except (ValueError, TypeError, AttributeError, IndexError):
        observed = False
    sys.exit(0 if observed else 1)

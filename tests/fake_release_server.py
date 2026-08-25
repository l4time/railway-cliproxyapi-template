#!/usr/bin/env python3
import hashlib
import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

ROOT = Path(os.environ["FIXTURE_ROOT"])
PORT = int(os.environ["FIXTURE_PORT"])
PUBLIC_HOST = os.environ.get("FIXTURE_PUBLIC_HOST", "host.docker.internal")
ARCH = os.environ.get("FIXTURE_ARCH", "aarch64")


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        return

    def do_GET(self):
        scenario = (ROOT / "scenario").read_text().strip()
        candidate_tag = (ROOT / "tag").read_text().strip()
        archive = ROOT / "candidate.tar.gz"
        checksum = hashlib.sha256(archive.read_bytes()).hexdigest()
        if scenario == "transient":
            self.send_response(503)
            self.end_headers()
            return
        if self.path == "/releases":
            tag = candidate_tag
            body = [{
                "tag_name": tag,
                "draft": False,
                "prerelease": False,
                "published_at": "2026-08-24T00:00:00Z",
                "assets": [
                    {
                        "name": f"CLIProxyAPI_{tag[1:]}_linux_{ARCH}.tar.gz",
                        "size": archive.stat().st_size,
                        "browser_download_url": f"http://{PUBLIC_HOST}:{PORT}/candidate.tar.gz",
                    },
                    {
                        "name": "checksums.txt",
                        "size": 96,
                        "browser_download_url": f"http://{PUBLIC_HOST}:{PORT}/checksums.txt",
                    },
                ],
            }]
            payload = json.dumps(body).encode()
        elif self.path == "/checksums.txt":
            if scenario == "bad-checksum":
                checksum = "0" * 64
            if scenario == "same-tag-drift":
                checksum = "f" * 64
            asset_tag = candidate_tag
            payload = f"{checksum}  CLIProxyAPI_{asset_tag[1:]}_linux_{ARCH}.tar.gz\n".encode()
        elif self.path == "/candidate.tar.gz":
            payload = archive.read_bytes()
        else:
            self.send_response(404)
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)


ThreadingHTTPServer(("0.0.0.0", PORT), Handler).serve_forever()

"""Runs the built UI (frontend/dist) in a headless browser against a mock backend
and fails on any JavaScript error.

The UI is plain JavaScript with no type checker, so a mistake such as a local
variable shadowing the t() translation function only shows up when that code
runs. This sweep renders every tab in both languages, in WARP and own-server
mode, with each kind of WARP account and connection state, clicks through the
controls that render more text, and collects window errors and rejections.

    cd frontend && npm run build && python test/ui_sweep.py
"""
import functools
import http.server
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import threading
from pathlib import Path

HERE = Path(__file__).resolve().parent
DIST = HERE.parent / "dist"

BROWSERS = [
    r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
    r"C:\Program Files\Microsoft\Edge\Application\msedge.exe",
    r"C:\Program Files\Google\Chrome\Application\chrome.exe",
    "/usr/bin/google-chrome", "/usr/bin/chromium", "/usr/bin/microsoft-edge",
]

CASES = [
    "mode=warp&acct=free", "mode=warp&acct=free0", "mode=warp&acct=none",
    "mode=warp&acct=unlimited&ep=1", "mode=warp&acct=limited&run=1",
    "mode=warp&acct=other&run=1&state=reconnecting", "mode=warp&acct=free&run=1&state=blocked",
    "mode=custom&acct=free&run=1&pending=1", "mode=custom&acct=none&err=1",
    "mode=warp&acct=free&admin=0&bins=0", "mode=warp&acct=free&admin=0&os=linux",
    "mode=custom&tab=vps", "mode=custom&tab=vps&vps=none", "mode=custom&tab=vps&vps=hk",
    "mode=custom&tab=vps&vps=failed", "mode=warp&acct=free&tab=route",
    "mode=warp&acct=free&tab=app&os=linux", "mode=warp&acct=free&tab=tools",
]


def browser() -> str:
    for b in BROWSERS:
        if os.path.exists(b):
            return b
    sys.exit("no Edge / Chrome found")


def main() -> int:
    if not (DIST / "index.html").exists():
        sys.exit("frontend/dist is missing: run npm run build first")
    with tempfile.TemporaryDirectory() as tmp:
        site = Path(tmp) / "site"
        shutil.copytree(DIST, site)
        shutil.copyfile(HERE / "ui_sweep_mock.js", site / "mock.js")
        index = site / "index.html"
        index.write_text(index.read_text(encoding="utf-8").replace(
            '<script type="module"', '<script src="/mock.js"></script><script type="module"', 1), encoding="utf-8")

        handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=str(site))
        handler.log_message = lambda *a: None
        srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
        threading.Thread(target=srv.serve_forever, daemon=True).start()
        base = f"http://127.0.0.1:{srv.server_address[1]}/index.html"

        failed = 0
        total = 0
        for lang in ("vi", "en"):
            for case in CASES:
                total += 1
                url = f"{base}?lang={lang}&{case}"
                out = ""
                for _ in range(3):  # a headless browser occasionally hangs
                    try:
                        out = subprocess.run(
                            [browser(), "--headless", "--disable-gpu", "--virtual-time-budget=5000", "--dump-dom", url],
                            capture_output=True, timeout=40).stdout.decode("utf-8", "replace")
                        break
                    except subprocess.TimeoutExpired:
                        continue
                m = re.search(r'<script type="application/json" id="errs">(.*?)</script>', out, re.S)
                errs = json.loads(m.group(1))["errs"] if m else ["the page did not finish loading"]
                if errs:
                    failed += 1
                    print(f"FAIL [{lang}] {case}: {errs}")
        srv.shutdown()
    print(f"{total} cases, {failed} failed")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())

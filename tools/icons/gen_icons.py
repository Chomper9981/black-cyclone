"""Render the app icons from logo/black-cyclone-logo.svg.

Uses Microsoft Edge / Chrome in headless mode to rasterise the SVG (no Python
packages needed) and packs the PNGs into .ico files itself.

    python tools/icons/gen_icons.py

Writes:
  build/appicon.png               1024 px, Wails app icon (Linux window icon, macOS)
  build/windows/icon.ico          16-256 px, exe + installer
  linux/blackcyclone.png          256 px, menu entry
  internal/tray/connected.{png,ico}     tray icon while connected (full colour)
  internal/tray/disconnected.{png,ico}  tray icon while disconnected (greyed)
  frontend/src/assets/logo.svg    copy of the logo for the UI
"""
import os
import shutil
import struct
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
LOGO = ROOT / "logo" / "black-cyclone-logo.svg"

BROWSERS = [
    r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
    r"C:\Program Files\Microsoft\Edge\Application\msedge.exe",
    r"C:\Program Files\Google\Chrome\Application\chrome.exe",
    "/usr/bin/google-chrome", "/usr/bin/chromium", "/usr/bin/microsoft-edge",
]

# Tray "disconnected": the same logo, desaturated and dimmed.
GREY = "filter: grayscale(1) brightness(1.6) opacity(.75);"


def browser() -> str:
    for b in BROWSERS:
        if os.path.exists(b):
            return b
    sys.exit("không tìm thấy Edge / Chrome để vẽ icon")


def render(size: int, out: Path, style: str = "", pad: float = 0.0) -> bytes:
    """Rasterises the logo to a size x size PNG with a transparent background."""
    inner = round(size * (1 - 2 * pad))
    off = (size - inner) // 2
    with tempfile.TemporaryDirectory() as tmp:
        page = Path(tmp) / "icon.html"
        page.write_text(
            '<!DOCTYPE html><html style="background:transparent"><body style="margin:0;background:transparent">'
            f'<img src="{LOGO.as_uri()}" width="{inner}" height="{inner}" '
            f'style="display:block;margin:{off}px 0 0 {off}px;{style}"></body></html>',
            encoding="utf-8")
        png = Path(tmp) / "icon.png"
        subprocess.run([
            browser(), "--headless", "--disable-gpu", "--hide-scrollbars",
            "--allow-file-access-from-files", "--default-background-color=00000000",
            f"--window-size={size},{size}", f"--screenshot={png}", page.as_uri(),
        ], check=True, capture_output=True, timeout=60)
        data = png.read_bytes()
    if out is not None:
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_bytes(data)
    return data


def ico(out: Path, images: dict[int, bytes]) -> None:
    """Packs PNG images into an .ico (PNG-compressed entries, Windows Vista+)."""
    sizes = sorted(images)
    header = struct.pack("<HHH", 0, 1, len(sizes))
    entries, blobs = b"", b""
    offset = 6 + 16 * len(sizes)
    for s in sizes:
        data = images[s]
        dim = 0 if s >= 256 else s
        entries += struct.pack("<BBBBHHII", dim, dim, 0, 0, 1, 32, len(data), offset)
        blobs += data
        offset += len(data)
    out.write_bytes(header + entries + blobs)


def main() -> None:
    if not LOGO.exists():
        sys.exit(f"thiếu {LOGO}")
    render(1024, ROOT / "build" / "appicon.png")
    render(256, ROOT / "linux" / "blackcyclone.png")
    ico(ROOT / "build" / "windows" / "icon.ico",
        {s: render(s, None) for s in (16, 20, 24, 32, 40, 48, 64, 128, 256)})

    tray = ROOT / "internal" / "tray"
    for name, style in (("connected", ""), ("disconnected", GREY)):
        render(48, tray / f"{name}.png", style)
        ico(tray / f"{name}.ico", {s: render(s, None, style) for s in (16, 20, 24, 32, 48)})

    shutil.copyfile(LOGO, ROOT / "frontend" / "src" / "assets" / "logo.svg")
    print("đã tạo icon từ", LOGO.relative_to(ROOT))


if __name__ == "__main__":
    main()

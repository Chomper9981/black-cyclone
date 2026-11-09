"""Generate the Black Cyclone system diagram (vi + en) as standalone SVG files.

Usage: python docs/tools/gen_diagram.py docs/assets
"""
import sys
from pathlib import Path

OUT = Path(sys.argv[1])

T = {
    "vi": {
        "aria": "Sơ đồ: thiết bị kết nối mã hoá qua nhà mạng tới VPS của bạn, VPS đi ra Internet; web Việt Nam đi thẳng",
        "dev": ("Thiết bị của bạn", "Black Cyclone · TUN", "kill switch"),
        "isp": "Nhà mạng / Wi-Fi",
        "isp_sees": ("chỉ thấy:", "HTTPS tới microsoft.com"),
        "vps": ("VPS của bạn", "sing-box · Reality", "khoá do bạn giữ"),
        "net": ("Internet", "website thấy", "IP của VPS"),
        "a1": "VLESS + Reality (mã hoá)",
        "a2": "ra ngoài",
        "direct": "Web Việt Nam, LAN: đi thẳng (chia tuyến)",
        "probe": ("ai dò thử chỉ nhận về", "trang thật của microsoft.com"),
    },
    "en": {
        "aria": "Diagram: your device connects, encrypted, through the ISP to your own VPS, which exits to the Internet; local sites go direct",
        "dev": ("Your device", "Black Cyclone · TUN", "kill switch"),
        "isp": "ISP / Wi-Fi",
        "isp_sees": ("only sees:", "HTTPS to microsoft.com"),
        "vps": ("Your VPS", "sing-box · Reality", "keys you hold"),
        "net": ("Internet", "sites see", "your VPS's IP"),
        "a1": "VLESS + Reality (encrypted)",
        "a2": "exit",
        "direct": "Local sites, LAN: go direct (split routing)",
        "probe": ("probes only get back", "the real microsoft.com page"),
    },
}

BG, SURF, LINE, TEXT, MUTED, ACC = "#131417", "#1a1b1f", "#4a4d54", "#ececee", "#9a9da4", "#ff6f6b"
FONT = "Segoe UI, system-ui, -apple-system, Roboto, sans-serif"


def box(x, y, w, h, lines, stroke=LINE, title_color=TEXT):
    cx = x + w / 2
    t, l1, l2 = lines
    return f"""
  <rect x="{x}" y="{y}" width="{w}" height="{h}" rx="12" fill="{SURF}" stroke="{stroke}" stroke-width="1.5"/>
  <text x="{cx}" y="{y + 44}" text-anchor="middle" font-size="16" font-weight="700" fill="{title_color}">{t}</text>
  <text x="{cx}" y="{y + 72}" text-anchor="middle" font-size="13" fill="{MUTED}">{l1}</text>
  <text x="{cx}" y="{y + 92}" text-anchor="middle" font-size="13" fill="{MUTED}">{l2}</text>"""


def svg(lang):
    s = T[lang]
    return f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 420" role="img" aria-label="{s['aria']}" font-family="{FONT}">
  <defs>
    <marker id="ah" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse">
      <path d="M0,0 L10,5 L0,10 z" fill="{TEXT}"/>
    </marker>
    <marker id="aha" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
      <path d="M0,0 L10,5 L0,10 z" fill="{ACC}"/>
    </marker>
    <marker id="ahm" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse">
      <path d="M0,0 L10,5 L0,10 z" fill="{MUTED}"/>
    </marker>
  </defs>
  <rect width="1000" height="420" rx="16" fill="{BG}"/>

  <!-- ISP zone -->
  <rect x="250" y="56" width="230" height="250" rx="12" fill="none" stroke="{LINE}" stroke-width="1.5" stroke-dasharray="6 6"/>
  <text x="365" y="84" text-anchor="middle" font-size="13" font-weight="600" fill="{MUTED}" letter-spacing=".5">{s['isp']}</text>
  <text x="365" y="236" text-anchor="middle" font-size="13" fill="{MUTED}">{s['isp_sees'][0]}</text>
  <text x="365" y="256" text-anchor="middle" font-size="13" fill="{TEXT}">{s['isp_sees'][1]}</text>
{box(30, 110, 180, 120, s['dev'])}
{box(520, 110, 180, 120, s['vps'], stroke=ACC, title_color=ACC)}
{box(790, 110, 180, 120, s['net'])}

  <!-- device -> VPS (encrypted tunnel) -->
  <line x1="210" y1="170" x2="516" y2="170" stroke="{ACC}" stroke-width="3" marker-end="url(#aha)"/>
  <text x="365" y="158" text-anchor="middle" font-size="13" font-weight="600" fill="{ACC}">{s['a1']}</text>

  <!-- VPS -> Internet -->
  <line x1="700" y1="170" x2="786" y2="170" stroke="{TEXT}" stroke-width="2" marker-end="url(#ah)"/>
  <text x="745" y="158" text-anchor="middle" font-size="13" fill="{MUTED}">{s['a2']}</text>

  <!-- probe note under VPS -->
  <text x="610" y="256" text-anchor="middle" font-size="12" fill="{MUTED}" font-style="italic">{s['probe'][0]}</text>
  <text x="610" y="274" text-anchor="middle" font-size="12" fill="{MUTED}" font-style="italic">{s['probe'][1]}</text>

  <!-- direct route -->
  <polyline points="120,230 120,360 880,360 880,234" fill="none" stroke="{MUTED}" stroke-width="1.5" stroke-dasharray="5 5" marker-end="url(#ahm)"/>
  <rect x="{500 - len(s['direct']) * 3.4 - 12}" y="348" width="{len(s['direct']) * 6.8 + 24}" height="24" fill="{BG}"/>
  <text x="500" y="365" text-anchor="middle" font-size="13" fill="{MUTED}">{s['direct']}</text>
</svg>
"""


for lang in T:
    (OUT / f"diagram-{lang}.svg").write_text(svg(lang), encoding="utf-8")
    print("wrote", OUT / f"diagram-{lang}.svg")

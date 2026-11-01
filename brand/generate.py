#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Netleash contributors
"""Generates the Netleash brand assets (SVG sources). PNG/ICNS are rendered by render.sh."""
import os, sys

OUT = os.path.dirname(os.path.abspath(__file__))

# ---- palette ----------------------------------------------------------------------------------------------------
NAVY, NAVY_2, NAVY_3 = "#0B1B33", "#10284A", "#08152A"
TEAL, TEAL_D, MINT = "#14B8A6", "#0D8F82", "#5EEAD4"
CORAL, AMBER, GREEN = "#FF5D73", "#FFB020", "#2DD4A0"
MIST, SLATE, WHITE = "#E6F0F5", "#7B8BA3", "#FFFFFF"

def svg(w, h, body, defs=""):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}">'
            f'<defs>{defs}</defs>{body}</svg>\n')

def lin(id_, c1, c2, x1=0, y1=0, x2=1, y2=1):
    return (f'<linearGradient id="{id_}" x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}">'
            f'<stop offset="0" stop-color="{c1}"/><stop offset="1" stop-color="{c2}"/></linearGradient>')

# ---- the mark: the letter N drawn as one continuous leash line between a handle and a node ---------------------------
# Geometry on a 512 x 512 grid. VARIANT "A": handle ring at the start of the line. VARIANT "C": plain monogram.
VARIANT = os.environ.get("NETLEASH_MARK", "A")

def mark(lead="#FFFFFF", node=MINT, ring=MINT, ring_opacity=0.55, sw=40, variant=None):
    v = variant or VARIANT
    top = 128
    start = 360 if v == "A" else 384
    body = (
        f'<circle cx="382" cy="{top}" r="90" fill="none" stroke="{ring}" stroke-opacity="{ring_opacity}" stroke-width="{sw*0.45:.0f}"/>'
        f'<path d="M 130 {start} L 130 {top} L 382 384 L 382 {top}" fill="none" stroke="{lead}" stroke-width="{sw}" '
        f'stroke-linecap="round" stroke-linejoin="round"/>'
        f'<circle cx="382" cy="{top}" r="46" fill="{node}"/>'
    )
    if v == "A":
        body += f'<circle cx="130" cy="400" r="40" fill="none" stroke="{lead}" stroke-width="{sw*0.85:.0f}"/>'
    return f'<g transform="translate(0 14)">{body}</g>'

def place(inner, size, x, y, box=512):
    s = size / box
    return f'<g transform="translate({x} {y}) scale({s:.5f})">{inner}</g>'

# ---- app icon (macOS grid: 824 px body centred on a 1024 canvas) --------------------------------------------------
def app_icon():
    defs = lin("bg", NAVY_2, NAVY_3, 0.2, 0, 0.8, 1) + \
           '<radialGradient id="glow" cx="0.72" cy="0.22" r="0.7"><stop offset="0" stop-color="#14B8A6" stop-opacity="0.55"/>' \
           '<stop offset="1" stop-color="#14B8A6" stop-opacity="0"/></radialGradient>'
    body = (f'<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#bg)"/>'
            f'<rect x="100" y="100" width="824" height="824" rx="185" fill="url(#glow)"/>'
            + place(mark(sw=44), 640, 192, 186))
    return svg(1024, 1024, body, defs)

# ---- plain mark (transparent) --------------------------------------------------------------------------------------
def mark_svg(lead=NAVY, node=TEAL, ring=TEAL, ring_opacity=0.55, bg=None):
    body = (f'<rect width="512" height="512" fill="{bg}"/>' if bg else "") + mark(lead, node, ring, ring_opacity)
    return svg(512, 512, body)

# ---- monoline wordmark "netleash" (drawn with strokes, no font needed) --------------------------------------------
BASE, XTOP, ASC = 100, 30, 0   # baseline, x-height line, ascender line (y grows downward)
def letter(ch, x):
    """Returns (path_d, advance). x is the left edge of the letter."""
    r = 35
    if ch == "n":
        return f"M {x} {BASE} V {XTOP} M {x} 62 A 28 28 0 0 1 {x+56} 62 V {BASE}", 56
    if ch == "h":
        return f"M {x} {BASE} V {ASC} M {x} 62 A 28 28 0 0 1 {x+56} 62 V {BASE}", 56
    if ch == "l":
        return f"M {x} {BASE} V {ASC}", 0
    if ch == "t":
        return f"M {x} 8 V 86 Q {x} {BASE} {x+18} {BASE} M {x-16} {XTOP+6} H {x+24}", 24
    if ch == "e":
        cx, cy = x + r, 65
        return f"M {x} {cy} H {x+2*r} A {r} {r} 0 1 0 {cx+25} {cy+25}", 2 * r
    if ch == "a":
        cx, cy = x + r, 65
        return (f"M {x} {cy} A {r} {r} 0 1 1 {x+2*r} {cy} A {r} {r} 0 1 1 {x} {cy} M {x+2*r} {XTOP} V {BASE}", 2 * r)
    if ch == "s":
        return (f"M {x+52} 44 C {x+44} 30 {x+4} 28 {x+4} 49 C {x+4} 68 {x+52} 60 {x+52} 82 "
                f"C {x+52} 104 {x+10} 102 {x} 88", 56)
    raise ValueError(ch)

def wordmark_paths(text="netleash", gap=26, stroke=13, color=NAVY, accent=TEAL, accent_char="l", x0=0):
    x, parts = x0, []
    for ch in text:
        d, adv = letter(ch, x if ch != "t" else x + 16)
        c = accent if ch == accent_char else color
        parts.append(f'<path d="{d}" fill="none" stroke="{c}" stroke-width="{stroke}" stroke-linecap="round" stroke-linejoin="round"/>')
        x += adv + gap + (14 if ch == "t" else 0)
    return "".join(parts), x - gap

def horizontal(dark=False):
    fg = WHITE if dark else NAVY
    wm, wm_w = wordmark_paths(color=fg, accent=MINT if dark else TEAL)
    scale = 1.35
    mark_size = 150
    gap = 40
    W = int(mark_size + gap + wm_w * scale + 20)
    H = 190
    inner = (place(mark(lead=fg, node=MINT if dark else TEAL, ring=MINT if dark else TEAL), mark_size, 0, 20)
             + f'<g transform="translate({mark_size+gap} {35}) scale({scale})">{wm}</g>')
    return svg(W, H, inner)

# ---- menu bar glyphs (18 pt, state colours; simplified mark) ---------------------------------------------------------
def menubar(color, slash=False, warn=False):
    """slash=True draws a CUT line (gap in the diagonal) = VPN down, app blocked."""
    top = 128
    if slash:
        line = (f'<path d="M 130 {384} L 130 {top} L 225 {top+97}" fill="none" stroke="{color}" stroke-width="56" '
                f'stroke-linecap="round" stroke-linejoin="round"/>'
                f'<path d="M 287 {top+161} L 382 384 L 382 {top}" fill="none" stroke="{color}" stroke-width="56" '
                f'stroke-linecap="round" stroke-linejoin="round"/>')
    else:
        line = (f'<path d="M 130 384 L 130 {top} L 382 384 L 382 {top}" fill="none" stroke="{color}" stroke-width="56" '
                f'stroke-linecap="round" stroke-linejoin="round"/>')
    body = (f'<circle cx="382" cy="{top}" r="88" fill="none" stroke="{color}" stroke-opacity="0.5" stroke-width="28"/>' + line +
            f'<circle cx="382" cy="{top}" r="46" fill="{color}"/>')
    return svg(512, 512, f'<g transform="translate(0 30)">{body}</g>')

# ---- GitHub social preview 1280 x 640 -----------------------------------------------------------------------------
def social():
    defs = lin("bg", NAVY_2, NAVY_3, 0.1, 0, 0.9, 1) + \
           '<radialGradient id="glow" cx="0.22" cy="0.4" r="0.6"><stop offset="0" stop-color="#14B8A6" stop-opacity="0.45"/>' \
           '<stop offset="1" stop-color="#14B8A6" stop-opacity="0"/></radialGradient>'
    wm, wm_w = wordmark_paths(color=WHITE, accent=MINT, stroke=13)
    body = ('<rect width="1280" height="640" fill="url(#bg)"/><rect width="1280" height="640" fill="url(#glow)"/>'
            + place(mark(sw=40), 380, 90, 130)
            + f'<g transform="translate(540 215) scale({640/wm_w:.3f})">{wm}</g>'
            + f'<text x="544" y="500" font-family="Helvetica Neue, Helvetica, Arial, sans-serif" font-size="34" fill="{SLATE}">'
              'A leash for your app’s network.</text>'
            + f'<text x="544" y="550" font-family="Helvetica Neue, Helvetica, Arial, sans-serif" font-size="26" fill="{TEAL}">'
              'Kernel-level · fail-closed · macOS</text>')
    return svg(1280, 640, body, defs)

# ---- contact sheet for review -----------------------------------------------------------------------------------------
def sheet():
    W, H = 1600, 1320
    defs = lin("bgi", NAVY_2, NAVY_3, 0.2, 0, 0.8, 1) + \
           '<radialGradient id="glowi" cx="0.72" cy="0.22" r="0.7"><stop offset="0" stop-color="#14B8A6" stop-opacity="0.55"/>' \
           '<stop offset="1" stop-color="#14B8A6" stop-opacity="0"/></radialGradient>'
    b = [f'<rect width="{W}" height="{H}" fill="#F4F8FA"/>']
    def label(x, y, t, color=SLATE):
        return f'<text x="{x}" y="{y}" font-family="Helvetica Neue, Helvetica, Arial, sans-serif" font-size="20" fill="{color}">{t}</text>'
    # app icon big + small sizes
    b.append(label(40, 44, "App icon"))
    b.append(f'<g transform="translate(20 50) scale(0.42)">{app_icon_body()}</g>')
    for i, s in enumerate((128, 64, 32, 16)):
        b.append(f'<g transform="translate({480 + sum((128,64,32,16)[:i])*1.4:.0f} 130) scale({s/1024:.4f})">{app_icon_body()}</g>')
    b.append(label(480, 290, "128 / 64 / 32 / 16 px"))
    # mark on light and dark
    b.append(label(40, 520, "Mark (light / dark / brand)"))
    b.append(f'<rect x="40" y="540" width="240" height="240" rx="24" fill="#FFFFFF" stroke="#D5E1E8"/>')
    b.append(place(mark(lead=NAVY, node=TEAL, ring=TEAL), 180, 70, 570))
    b.append(f'<rect x="300" y="540" width="240" height="240" rx="24" fill="{NAVY}"/>')
    b.append(place(mark(lead=WHITE, node=MINT, ring=MINT), 180, 330, 570))
    b.append(f'<rect x="560" y="540" width="240" height="240" rx="24" fill="{TEAL}"/>')
    b.append(place(mark(lead=WHITE, node=NAVY, ring=NAVY, ring_opacity=0.5), 180, 590, 570))
    # horizontal logos
    b.append(label(840, 520, "Horizontal logo"))
    b.append(f'<rect x="840" y="540" width="720" height="110" rx="20" fill="#FFFFFF" stroke="#D5E1E8"/>')
    b.append(f'<g transform="translate(860 550) scale(0.5)">{inner(horizontal(False))}</g>')
    b.append(f'<rect x="840" y="670" width="720" height="110" rx="20" fill="{NAVY}"/>')
    b.append(f'<g transform="translate(860 680) scale(0.5)">{inner(horizontal(True))}</g>')
    # menu bar states
    b.append(label(40, 840, "Menu bar states (18 pt, shown at 3x on a light and a dark bar)"))
    states = [("VPN up", GREEN, False), ("VPN down", CORAL, True), ("Protection off", AMBER, False)]
    for k, (name, col, slash) in enumerate(states):
        x = 40 + k * 260
        b.append(f'<rect x="{x}" y="860" width="240" height="64" rx="12" fill="#FFFFFF" stroke="#D5E1E8"/>')
        b.append(f'<rect x="{x}" y="936" width="240" height="64" rx="12" fill="#1B1F27"/>')
        for yy in (870, 946):
            b.append(f'<g transform="translate({x+18} {yy}) scale({44/512:.4f})">{inner(menubar(col, slash))}</g>')
            b.append(label(x + 76, yy + 36, name, "#46566B" if yy == 870 else "#C8D3E0"))
    # social preview
    b.append(label(840, 840, "GitHub social preview (1280 x 640)"))
    b.append(f'<g transform="translate(840 860) scale(0.5625)">{inner(social())}</g>')
    return svg(W, H, "".join(b), defs)

def inner(s):
    """Strips the <svg> wrapper so a document can be embedded as a group (ids must stay unique per document)."""
    a = s.index(">") + 1
    d0 = s.index("<defs>")
    d1 = s.index("</defs>") + 7
    return s[d0:d1].replace("<defs></defs>", "") + s[d1:s.rindex("</svg>")] if "<defs></defs>" not in s[d0:d1] else s[d1:s.rindex("</svg>")]

def app_icon_body():
    return inner(app_icon()).replace('id="bg"', 'id="bgi"').replace('url(#bg)', 'url(#bgi)') \
                            .replace('id="glow"', 'id="glowi"').replace('url(#glow)', 'url(#glowi)')

FILES = {
    "icon.svg": app_icon(),
    "mark.svg": mark_svg(),
    "mark-dark.svg": mark_svg(WHITE, MINT, MINT),
    "logo-horizontal.svg": horizontal(False),
    "logo-horizontal-dark.svg": horizontal(True),
    "menubar-up.svg": menubar(GREEN),
    "menubar-down.svg": menubar(CORAL, slash=True),
    "menubar-warn.svg": menubar(AMBER),
    "social-preview.svg": social(),
    "_sheet.svg": sheet(),
}
if __name__ == "__main__":
    os.makedirs(os.path.join(OUT, "svg"), exist_ok=True)
    for name, content in FILES.items():
        with open(os.path.join(OUT, "svg", name), "w") as f:
            f.write(content)
    print("wrote", len(FILES), "svg files")

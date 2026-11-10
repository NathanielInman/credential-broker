#!/usr/bin/env python3
"""Writes artwork/hero.svg: the README's animated hero.

One loop in two acts. First broker: an agent in the terminal asks for
CLOUDFLARE_API_TOKEN, the request window springs up, a YubiKey touch grants
it, and the deploy runs with the token redacted from its output. Then Signal
Box: the lamp in waybar counts grants as they arrive, a click opens the panel
out of that icon, a look at the Asks tab, Revoke all, and the panel folds back
into the lamp.

Pure CSS animation inside the SVG, so GitHub plays it in an <img>. With
reduced motion the art is a still of the open panel.

    python3 artwork/hero/hero.py                 # write artwork/hero.svg
    python3 artwork/hero/hero.py --still 13.5    # a frozen frame, for checking
    python3 artwork/hero/hero.py --still 13.5 --out /tmp/frame.svg

How it is put together
----------------------
Objects that live across scenes (the desk, the terminal, the request window,
the bar and its lamp, the panel, the cursor) are drawn once by the build_*
functions, each animated part with an id. Nothing in them moves by itself.

SCENES is the timeline: (name, seconds, function). The loop length is the
sum of the seconds, and start(name) is where a scene begins. Each scene
function gets its own start time t0 and length d and adds keyframes to the
objects with key(), show(), spring_open() and friends, using absolute times
(t0 + something). compile_css() turns every object's keys into one CSS
@keyframes over the whole loop, so percentages are never written by hand.

To add a scene: write `def scene_name(t0, d): ...`, put
("name", seconds, scene_name) in SCENES where it belongs, and regenerate.
Everything after it shifts on its own. If the scene needs a new object, draw
it in a build_* function (or from the scene itself with add(layer, svg)) and
give each moving part an id. Leave every object at the end of the loop the
way it was at the start: the script warns when an object's first and last
frames differ, since that shows up as a jump when the loop restarts.
"""

import argparse
import math
from collections import defaultdict
from pathlib import Path

W, H = 1200, 680

# Nord, as in Roundhouse's and Switchyard's heroes and the default waybar.
BG0, BG1 = "#15181d", "#0d0f12"
BAR, BAR_FG, BAR_DIM, BAR_LINE = "#282a2e", "#c5c8c6", "#7c838e", "#3b4048"
PANEL, PANEL_LINE = "#23262b", "#3d424b"
TILE, TILE_LINE, STRIP = "#2c3036", "#393e46", "#1f2226"
INK, FG, DIM, MUTE = "#e8ebf0", "#c5c8c6", "#8d949f", "#666d78"
ACCENT, CALM, WARN, ACT, DARK = "#88c0d0", "#a3be8c", "#ebcb8b", "#bf616a", "#1d2024"
FROST2, BLUE = "#81a1c1", "#5e81ac"
TERM_BG, TERM_TOP = "#1a1d22", "#1f2328"

# The request window, in broker-dialog's own colors (broker-dialog/ui.html).
DLG_BG, DLG_FG, DLG_MUTED, DLG_FAINT = "#1b1c21", "#e9e9ee", "#9a9ca8", "#7d808c"
DLG_PANEL, DLG_LINE, DLG_BUTTON, DLG_PRIMARY = "#24262d", "#30323b", "#33353e", "#4f7ef7"
CLAIM_BG, CLAIM_LINE, CLAIM_FG = "#2a261e", "#c9a24a", "#d9b45c"
TOUCH_BG, TOUCH_LINE, OK = "#1d2a22", "#2f4c37", "#5fd38a"

FONT = "JetBrains Mono, DejaVu Sans Mono, Menlo, Consolas, monospace"
SANS = "Inter, Noto Sans, Cantarell, DejaVu Sans, Helvetica Neue, Arial, sans-serif"

# Geometry shared by several objects.
BAR_Y, BAR_H = 636, 44
BAR_TEXT_Y = BAR_Y + 27
LAMP_X, LAMP_Y = 1036, BAR_Y + BAR_H / 2
TX, TY, TW, TH = 36, 34, 664, 372           # terminal
RW, RH = 470, 470                            # request window
RX, RY = (W - RW) // 2, 80
PW, PH = 452, 520                            # Signal Box panel
PX = min(LAMP_X - PW // 2, W - 12 - PW)
PY = BAR_Y - 10 - PH
TAB_W, TAB_X0 = 86, 17                       # tab pills inside the panel
CURSOR_HOME = (600, 560)

# ---------------------------------------------------------------- markup

LAYERS = ["desk", "windows", "yubikey", "request", "bar", "panel", "overlay", "cursor"]
layers = defaultdict(list)


def add(layer, svg):
    layers[layer].append(svg)


def esc(s):
    return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace('"', "&quot;")


def attrs(id=None, cls=None, style=None, extra=""):
    """id, class and style attributes. An opacity in style becomes the
    presentation attribute, which the animation and the reduced-motion
    still can both override (an inline style would beat the still)."""
    out = ""
    if id:
        out += f' id="{id}"'
    if cls:
        out += f' class="{cls}"'
    if style:
        rest = []
        for d in style.split(";"):
            k, _, v = d.partition(":")
            if k.strip() == "opacity":
                out += f' opacity="{v.strip()}"'
            elif d.strip():
                rest.append(d.strip())
        if rest:
            out += f' style="{";".join(rest)}"'
    if extra:
        out += " " + extra
    return out


def text(x, y, s, size=13, fill=FG, weight=400, anchor="start", family=None, spacing=0,
         italic=False, id=None, cls=None, style=None):
    """One line of text. Returns the markup; callers add() it to a layer."""
    a = f' font-size="{size}" fill="{fill}"'
    if weight != 400:
        a += f' font-weight="{weight}"'
    if anchor != "start":
        a += f' text-anchor="{anchor}"'
    if family:
        a += f' font-family="{family}"'
    if spacing:
        a += f' letter-spacing="{spacing}"'
    if italic:
        a += ' font-style="italic"'
    return f'<text x="{x}" y="{y}"{a}{attrs(id, cls, style)}>{esc(s)}</text>'


def spans(x, y, parts, size=13, family=None, id=None, style=None):
    """A line in several colors: parts are (text, fill) or (text, fill, weight)."""
    fam = f' font-family="{family}"' if family else ""
    inner = ""
    for p in parts:
        s, fill = p[0], p[1]
        w = f' font-weight="{p[2]}"' if len(p) > 2 else ""
        inner += f'<tspan fill="{fill}"{w}>{esc(s)}</tspan>'
    return f'<text x="{x}" y="{y}" font-size="{size}"{fam}{attrs(id, None, style)}>{inner}</text>'


def rect(x, y, w, h, rx=0, fill="none", stroke=None, id=None, cls=None, style=None, extra=""):
    s = f' stroke="{stroke}"' if stroke else ""
    return f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{rx}" fill="{fill}"{s}{attrs(id, cls, style, extra)}/>'


def mono_w(s, size):
    """Approximate width of s in the monospace stack (0.6em a glyph)."""
    return len(s) * size * 0.6


# ---------------------------------------------------------------- the timeline

# tracks[id] is a list of (seconds, {css property: value}, timing function).
tracks = defaultdict(list)


def key(el, t, ease=None, **props):
    """At absolute time t, el has props (underscores become hyphens). ease is
    the timing function for the stretch that starts at this key."""
    tracks[el].append((t, {k.replace("_", "-"): v for k, v in props.items()}, ease))


def hold(el, t0, t1, **props):
    key(el, t0, **props)
    key(el, t1, **props)


def show(el, t_in, t_out=None, fade=0.25, hidden="0", shown="1"):
    """Fades el in at t_in and, with t_out, out again: opacity only."""
    key(el, t_in, opacity=hidden)
    key(el, t_in + fade, opacity=shown)
    if t_out is not None:
        key(el, t_out, opacity=shown)
        key(el, t_out + fade, opacity=hidden)


def swap(old, new, t, fade=0.22):
    """Crossfades from old to new at t."""
    key(old, t, opacity="1")
    key(old, t + fade, opacity="0")
    key(new, t, opacity="0")
    key(new, t + fade, opacity="1")


def spring_open(el, t, dur=0.72, dy=26, from_scale=0.06):
    """Springs el open from its transform-origin, overshooting a little."""
    f = lambda a: t + a * dur
    key(el, f(0), transform=f"translateY({dy}px) scale({from_scale})", opacity="0")
    key(el, f(0.14), transform=f"translateY({dy * 0.55:.1f}px) scale({(from_scale + 1) / 2:.3f})", opacity=".85")
    key(el, f(0.28), transform=f"translateY({dy * 0.15:.1f}px) scale(.92)", opacity="1")
    key(el, f(0.42), transform="translateY(-1px) scale(1.03)")
    key(el, f(0.58), transform="translateY(0px) scale(.99)")
    key(el, f(0.78), transform="translateY(0px) scale(1.005)")
    key(el, f(1), transform="translateY(0px) scale(1)", opacity="1")


def spring_close(el, t, dur=0.32, dy=14, to_scale=0.4):
    """Folds el back into its transform-origin."""
    key(el, t, ease="cubic-bezier(.5,0,.75,0)", transform="translateY(0px) scale(1)", opacity="1")
    key(el, t + dur, transform=f"translateY({dy}px) scale({to_scale})", opacity="0")


def rise(el, t, dy=12, dur=0.4):
    """el arrives from a little below."""
    key(el, t, ease="cubic-bezier(.2,.8,.2,1)", transform=f"translateY({dy}px)", opacity="0")
    key(el, t + dur, transform="translateY(0px)", opacity="1")


ripples = []


def ripple(t, x, y, color=ACCENT):
    """A click: a ring that grows and fades where the cursor pressed."""
    rid = f"ripple{len(ripples)}"
    ripples.append(rid)
    add("overlay", f'<circle id="{rid}" cx="{x}" cy="{y}" r="18" fill="none" stroke="{color}" stroke-width="2.5" '
                   f'style="transform-box:fill-box;transform-origin:center"/>')
    key(rid, t - 0.01, opacity="0", transform="scale(.3)")
    key(rid, t, opacity=".9", transform="scale(.3)")
    key(rid, t + 0.45, opacity="0", transform="scale(1.7)")


class Cursor:
    """The pointer. go() glides it to a point, arriving at t; click() also
    leaves a ripple there."""

    def __init__(self, x, y):
        self.x, self.y = x, y

    def go(self, t, x, y, travel=0.55):
        key("cursor", t - travel, ease="cubic-bezier(.45,0,.25,1)", transform=f"translate({self.x}px,{self.y}px)")
        key("cursor", t, transform=f"translate({x}px,{y}px)")
        self.x, self.y = x, y

    def click(self, t, x, y, travel=0.55):
        self.go(t - 0.05, x, y, travel)
        ripple(t, x, y)


cursor = Cursor(*CURSOR_HOME)


def typed(layer, id, x, y, s, t0, t1, size=13, parts=None, clip="term-clip"):
    """A line typed out between t0 and t1: a cover the color of the terminal
    slides off it in character steps. Returns the cover's id."""
    w = mono_w(s, size) + 6
    line = spans(x, y, parts, size) if parts else text(x, y, s, size)
    add(layer, f'<g id="{id}" clip-path="url(#{clip})">{line}'
               f'{rect(x - 2, y - size, w + 4, size + 8, fill=TERM_BG, id=id + "-cover")}</g>')
    steps = max(1, len(s))
    key(id + "-cover", t0, ease=f"steps({steps}, end)", transform="translateX(0px)")
    key(id + "-cover", t1, transform=f"translateX({w + 4:.0f}px)")
    return id + "-cover"


# ---------------------------------------------------------------- objects

def build_desk():
    add("desk", f'<rect width="{W}" height="{H}" fill="url(#wall)"/>'
                f'<rect width="{W}" height="{H}" fill="url(#haze1)"/><rect width="{W}" height="{H}" fill="url(#haze2)"/>')
    # An editor behind, so the windows have something to sit over.
    x, y, w, h = 560, 66, 600, 300
    add("desk", rect(x, y, w, h, 12, "#191c21", "#2a2e35"))
    add("desk", rect(x, y, w, 30, 12, TERM_TOP))
    for k, c in enumerate(["#bf616a", "#ebcb8b", "#a3be8c"]):
        add("desk", f'<circle cx="{x + 18 + k * 16}" cy="{y + 15}" r="5" fill="{c}" fill-opacity=".45"/>')
    bars = [(0, .42), (24, .55), (48, .38), (24, .62), (0, .3), (24, .5), (48, .7), (48, .44), (24, .36), (0, .2)]
    cols = ["#3b4252", "#434c5e", "#5e81ac55", "#88c0d040", "#4c566a"]
    for i, (indent, f) in enumerate(bars):
        add("desk", rect(x + 22 + indent, y + 50 + i * 24, int(w * f), 9, 4, cols[i % len(cols)]))


def build_terminal():
    add("windows", f'<g filter="url(#shadow)">{rect(TX, TY, TW, TH, 12, TERM_BG, "#2b2f36")}</g>')
    add("windows", rect(TX, TY, TW, 30, 12, TERM_TOP))
    add("windows", rect(TX, TY + 18, TW, 12, 0, TERM_TOP))
    for k, c in enumerate(["#bf616a", "#ebcb8b", "#a3be8c"]):
        add("windows", f'<circle cx="{TX + 18 + k * 16}" cy="{TY + 15}" r="5" fill="{c}" fill-opacity=".6"/>')
    add("windows", text(TX + TW / 2, TY + 20, "claude · bubble-and-breakfast", 11.5, DIM, anchor="middle"))
    x = TX + 22
    add("windows", spans(x, line_y(0), [("~/Sites/bubble-and-breakfast/web", "#6b7380"), ("  main", ACCENT)]))
    add("windows", spans(x, line_y(1), [("❯ ", CALM), ("claude", FG)]))


def line_y(i):
    return TY + 58 + i * 22


def build_request():
    """broker-dialog's request window, as in artwork/request-window.png."""
    x, y = RX, RY
    g = [f'<g id="request" style="transform-origin:{RX + RW / 2}px {RY + RH / 2}px">',
         f'<g transform="translate({x},{y})" font-family="{SANS}">',
         f'<g filter="url(#shadow)">{rect(0, 0, RW, RH, 16, DLG_BG, "#2a2c33")}</g>',
         rect(22, 20, 38, 38, 10, DLG_PANEL),
         f'<g fill="none" stroke="{DLG_PRIMARY}" stroke-width="2" stroke-linecap="round">'
         f'<circle cx="35" cy="44" r="5"/><path d="M39 40 L50 29 M46 33 L49 36 M43 36 L45 38"/></g>',
         text(72, 37, "An agent wants to use 1 secret", 15, DLG_FG, 600),
         text(72, 55, "broker request · expires in 1:58", 12, DLG_MUTED),
         text(22, 90, "SECRETS", 11, DLG_FAINT, spacing=0.7),
         rect(22, 98, RW - 44, 32, 8, DLG_PANEL),
         text(32, 119, "CLOUDFLARE_API_TOKEN", 12, DLG_FG, family=FONT),
         text(RW - 32, 119, "Cloudflare API, DNS and Workers", 12, DLG_MUTED, anchor="end"),
         text(22, 158, "REQUEST", 11, DLG_FAINT, spacing=0.7)]
    k, v = 22, 114
    g += [text(k, 186, "For", 13, DLG_FAINT),
          rect(v, 168, 130, 26, 6, DLG_PANEL, DLG_LINE),
          text(v + 9, 186, "1 hour (asked)", 13, DLG_FG),
          f'<path d="M{v + 113} 179 l4 4 4-4" fill="none" stroke="{DLG_MUTED}" stroke-width="1.5"/>',
          text(v + 142, 186, "until 15:32", 13, DLG_MUTED)]
    for i, (label, val) in enumerate([("Asked by", "bash < zsh < claude < t3code"),
                                      ("Directory", "~/Sites/bubble-and-breakfast/web"),
                                      ("Will run", "npm run deploy")]):
        yy = 214 + i * 24
        g += [text(k, yy, label, 13, DLG_FAINT), text(v, yy, val, 12, DLG_FG, family=FONT)]
    g += [rect(22, 272, RW - 44, 52, 6, CLAIM_BG),
          rect(22, 272, 3, 52, 0, CLAIM_LINE),
          text(35, 291, "The agent says (unverified)", 11, CLAIM_FG),
          text(35, 311, "Deploying bnb.twinspear.com", 13, DLG_FG),
          text(22, 346, "While granted, any agent on this machine can use it.", 12, DLG_MUTED)]
    # The approve box, waiting for a touch, then granted.
    g += [f'<g id="touch-wait">',
          rect(22, 360, RW - 44, 58, 10, TOUCH_BG, TOUCH_LINE),
          f'<circle cx="44" cy="389" r="12" fill="none" stroke="{OK}" stroke-width="2" class="pulse" '
          f'style="transform-box:fill-box;transform-origin:center"/>',
          f'<circle cx="44" cy="389" r="6" fill="{OK}"/>',
          text(62, 385, "Touch your YubiKey to approve", 14, DLG_FG, 600),
          text(62, 403, "or press Deny to turn it down", 12, DLG_MUTED),
          "</g>",
          f'<g id="touch-done" opacity="0">',
          rect(22, 360, RW - 44, 58, 10, "#1f3327", "#3f7a52"),
          f'<circle cx="44" cy="389" r="11" fill="{OK}"/>',
          f'<path d="M38.5 389.5 l4 4 7.5 -8" fill="none" stroke="#10241a" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/>',
          text(62, 385, "Granted for 1 hour", 14, DLG_FG, 600),
          text(62, 403, "until 15:32 · the agent's command is running", 12, DLG_MUTED),
          "</g>",
          rect(RW - 90, 430, 68, 30, 8, DLG_BUTTON),
          text(RW - 56, 450, "Deny", 13, DLG_FG, anchor="middle"),
          "</g></g>"]
    add("request", "".join(g))


def build_yubikey():
    x, y = 872, 372
    add("yubikey", f'<g id="yubikey" style="transform-origin:{x + 24}px {y + 64}px">'
                   f'{rect(x + 15, y - 14, 18, 18, 3, "#9aa0a8")}'
                   f'<g filter="url(#shadow)">{rect(x, y, 48, 128, 10, "#202126", "#3a3b42")}</g>'
                   f'<circle cx="{x + 24}" cy="{y + 84}" r="14" fill="#2a2a30"/>'
                   f'<circle cx="{x + 24}" cy="{y + 84}" r="11" fill="#b48a32" class="blink"/>'
                   f'<circle cx="{x + 24}" cy="{y + 84}" r="6" fill="none" stroke="#e7c066" stroke-width="1.4" class="blink"/>'
                   f'<circle id="yk-flash" cx="{x + 24}" cy="{y + 84}" r="12" fill="none" stroke="#f5d47a" stroke-width="3" '
                   f'style="transform-box:fill-box;transform-origin:center"/>'
                   f'{text(x + 24, y + 36, "5C", 10, "#7c7f88", 700, "middle")}'
                   "</g>")


def lamp_arc_len():
    return 2 * math.pi * 9


def build_bar():
    b = []
    b.append(rect(0, BAR_Y, W, BAR_H, 0, BAR))
    b.append(f'<line x1="0" y1="{BAR_Y}" x2="{W}" y2="{BAR_Y}" stroke="#000" stroke-opacity=".4"/>')
    for i in range(6):
        x = 18 + i * 30
        if i == 3:
            b.append(rect(x - 4, BAR_Y + 9, 26, 26, 6, ACCENT))
        b.append(text(x + 9, BAR_TEXT_Y, str(i + 1), 14, DARK if i == 3 else BAR_DIM, 700 if i == 3 else 400, "middle"))
    b.append(spans(208, BAR_TEXT_Y, [("claude", BAR_FG), ("  ~/Sites/bubble-and-breakfast", BAR_DIM)], 14))
    for sx in (686, 824, 912, 960, 1010, 1092):
        b.append(f'<line x1="{sx}" y1="{BAR_Y + 10}" x2="{sx}" y2="{BAR_Y + BAR_H - 10}" stroke="{BAR_LINE}"/>')
    b.append(text(700, BAR_TEXT_Y, "♪ lo-fi radio", 14, BAR_DIM))
    b.append(text(838, BAR_TEXT_Y, "vol 42%", 14, BAR_DIM))
    # Roundhouse's lamp and Switchyard's signal, the siblings.
    b.append(f'<circle cx="936" cy="{LAMP_Y}" r="10" fill="{CALM}" fill-opacity=".18"/><circle cx="936" cy="{LAMP_Y}" r="6.5" fill="{CALM}"/>')
    b.append(rect(979, LAMP_Y - 11, 12, 22, 4, "#1d2024", "#444b55"))
    for k, c in enumerate(["#3b4048", "#3b4048", CALM]):
        b.append(f'<circle cx="985" cy="{LAMP_Y - 6 + k * 6}" r="2.2" fill="{c}"/>')
    # Signal Box's lamp: a quiet gray token, or a frost ring with a count.
    b.append(rect(LAMP_X - 18, BAR_Y + 7, 52, 30, 7, ACCENT, id="lamp-bg", style="opacity:0", extra='fill-opacity=".22"'))
    b.append(f'<circle cx="{LAMP_X}" cy="{LAMP_Y}" r="9" fill="none" stroke="#454b55" stroke-width="2.2"/>')
    b.append(f'<circle id="lamp-quiet" cx="{LAMP_X}" cy="{LAMP_Y}" r="3" fill="{MUTE}"/>')
    c = lamp_arc_len()
    b.append(f'<g id="lamp-live" opacity="0">'
             f'<circle id="lamp-arc" cx="{LAMP_X}" cy="{LAMP_Y}" r="9" fill="none" stroke="{ACCENT}" stroke-width="2.2" '
             f'stroke-linecap="round" stroke-dasharray="{c:.2f}" stroke-dashoffset="0" transform="rotate(-90 {LAMP_X} {LAMP_Y})"/>'
             f'<circle cx="{LAMP_X}" cy="{LAMP_Y}" r="4" fill="{ACCENT}"/></g>')
    for n in ("1", "2", "7"):
        b.append(text(LAMP_X + 16, BAR_TEXT_Y, n, 14, ACCENT, 700, id=f"count-{n}",
                      style="opacity:0;transform-box:fill-box;transform-origin:center"))
    b.append(text(W - 20, BAR_TEXT_Y, "14:32", 14, BAR_FG, anchor="end", id="clock-a"))
    b.append(text(W - 20, BAR_TEXT_Y, "15:20", 14, BAR_FG, anchor="end", id="clock-b", style="opacity:0"))
    add("bar", "".join(b))


def ring(cx, cy, frac, color, label, id):
    c = 2 * math.pi * 18
    return (f'<circle cx="{cx}" cy="{cy}" r="18" fill="none" stroke="#3b4048" stroke-width="4"/>'
            f'<circle id="{id}" cx="{cx}" cy="{cy}" r="18" fill="none" stroke="{color}" stroke-width="4" stroke-linecap="round" '
            f'stroke-dasharray="{c:.2f}" stroke-dashoffset="{c * (1 - frac):.2f}" transform="rotate(-90 {cx} {cy})"/>'
            + text(cx, cy + 4, label, 10.5, INK, 500, "middle"))


def spark_path(x, y, w, h, vals, color, id):
    mx = max(vals) or 1
    pts = [(x + i / (len(vals) - 1) * w, y + h - v / mx * (h - 2)) for i, v in enumerate(vals)]
    d = "M" + " L".join(f"{px:.1f} {py:.1f}" for px, py in pts)
    length = sum(math.dist(pts[i], pts[i + 1]) for i in range(len(pts) - 1)) + 4
    area = d + f" L{x + w} {y + h} L{x} {y + h} Z"
    return (f'<path d="{area}" fill="{color}" fill-opacity=".14"/>'
            f'<path id="{id}" d="{d}" fill="none" stroke="{color}" stroke-width="1.5" stroke-linejoin="round" '
            f'stroke-dasharray="{length:.0f}" stroke-dashoffset="0"/>'), length


# The three grants the panel shows, at 15:20.
GRANTS = [
    dict(id="g-cf", name="CLOUDFLARE_API_TOKEN", ring=(0.2, WARN, "12m"), who="claude · bubble-and-breakfast",
         why="deploying bnb.twinspear.com", use="6 runs · last 47m ago", spark=[0, 2, 3, 1, 0, 0, 0, 0, 0, 0, 0, 0]),
    dict(id="g-jira", name="JIRA_API_TOKEN", ring=(0.52, ACCENT, "31m"), who="claude · /tmp/dtcd-831",
         why="DTCD-704 QA: Jira evidence", use="23 runs · last 40s ago", spark=[0, 0, 1, 2, 3, 1, 4, 2, 3, 5, 2, 4]),
    dict(id="g-home", name="5 secrets", bundle="homelab", ring=(0.19, ACCENT, "1h42"), who="you · one touch at 08:02",
         why="sonarr radarr prowlarr sabnzbd truenas", use="211 runs · last 6m ago", spark=[5, 8, 6, 9, 7, 4, 6, 8, 5, 3, 2, 2]),
]
spark_lengths = {}
TINT = 'fill-opacity=".14"'


def build_panel():
    p = [f'<g id="panel" opacity="0" style="transform-origin:{LAMP_X}px {LAMP_Y}px">',
         f'<g transform="translate({PX},{PY})">',
         f'<g filter="url(#shadow)">{rect(0, 0, PW, PH, 14, PANEL, PANEL_LINE)}</g>']
    # Header
    p.append(f'<g id="p-head">'
             f'<circle cx="22" cy="28" r="4.5" fill="{CALM}"/>'
             + text(34, 33, "SIGNAL BOX", 14, INK, 700, spacing=1.7)
             + text(20, 53, "frostyarch · vault: hardware · YubiKey 5C", 12, DIM)
             + '<g id="chip-live">' + rect(PW - 86, 17, 68, 24, 12, ACCENT, extra=TINT)
             + text(PW - 52, 33, "7 live", 12, ACCENT, 500, "middle") + "</g>"
             + '<g id="chip-none" opacity="0">' + rect(PW - 118, 17, 100, 24, 12, CALM, extra=TINT)
             + text(PW - 68, 33, "nothing live", 12, CALM, 500, "middle") + "</g></g>")
    # Tabs
    p.append(f'<g id="p-tabs">{rect(14, 66, PW - 28, 32, 9, "#1d2024")}'
             + rect(TAB_X0, 69, TAB_W, 26, 6, ACCENT, id="tab-pill")
             + text(TAB_X0 + TAB_W / 2, 87, "Live 3", 12.5, DARK, 700, "middle", id="tab-live")
             + text(TAB_X0 + TAB_W / 2, 87, "Live 0", 12.5, DIM, 400, "middle", id="tab-live0", style="opacity:0")
             + text(TAB_X0 + TAB_W * 1.5, 87, "Asks 1", 12.5, DIM, 400, "middle", id="tab-asks")
             + text(TAB_X0 + TAB_W * 2.5, 87, "Register", 12.5, DIM, 400, "middle")
             + text(TAB_X0 + TAB_W * 3.5, 87, "Vault 38", 12.5, DIM, 400, "middle")
             + f'<circle cx="{TAB_X0 + TAB_W * 4 + 10}" cy="82.5" r="3.5" fill="{CALM}"/>'
             + text(TAB_X0 + TAB_W * 4 + 18, 87, "Yard ↗", 12.5, DIM)
             + "</g>")
    # Live view
    v = ['<g id="live-view">',
         f'<g id="p-strip">{rect(14, 110, PW - 28, 40, 9, STRIP, "#353a41")}'
         + spans(26, 135, [("7 secrets · 3 grants · next ends ", FG), ("12m", WARN)], 12.5)
         + rect(320, 118, 112, 24, 6, "#353a41", "#474d57", id="revoke-all-btn")
         + spans(332, 134, [("Revoke all ", FG), ("R", MUTE)], 12) + "</g>"]
    for i, g in enumerate(GRANTS):
        ty = 158 + i * 103
        frac, col, lab = g["ring"]
        sp, ln = spark_path(82, ty + 70, 84, 14, g["spark"], col, g["id"] + "-spark")
        spark_lengths[g["id"]] = ln
        name = (spans(82, ty + 25, [(" " + g["bundle"] + " ", ACCENT, 500), ("  " + g["name"], INK, 700)], 13)
                + rect(80, ty + 11, mono_w(g["bundle"], 13) + 14, 19, 4, "none", "#88c0d055")) if g.get("bundle") \
            else text(82, ty + 25, g["name"], 13, INK, 700)
        v.append(f'<g id="{g["id"]}">'
                 + rect(14, ty, PW - 28, 95, 10, TILE, TILE_LINE)
                 + ring(48, ty + 44, frac, col, lab, g["id"] + "-ring")
                 + name
                 + text(82, ty + 44, g["who"], 12, DIM)
                 + text(82, ty + 61, g["why"], 12, "#d8c79c" if not g.get("bundle") else MUTE, italic=not g.get("bundle"))
                 + sp
                 + text(176, ty + 82, g["use"], 11.5, MUTE)
                 + rect(362, ty + 12, 64, 24, 6, "#353a41", "#474d57") + text(394, ty + 28, "Revoke", 12, FG, anchor="middle")
                 + rect(362, ty + 42, 64, 24, 6, "#353a41", "#474d57") + text(394, ty + 58, "+1h", 12, FG, anchor="middle")
                 + "</g>")
    v.append(f'<g id="live-empty" opacity="0">{rect(14, 110, PW - 28, 110, 10, TILE, TILE_LINE)}'
             + text(PW / 2, 156, "Nothing is granted.", 13, INK, 700, "middle")
             + text(PW / 2, 180, "Agents that need a private secret will ask,", 12, DIM, anchor="middle")
             + text(PW / 2, 198, "and the lamp will tell you.", 12, DIM, anchor="middle") + "</g>")
    v.append("</g>")
    p += v
    # Asks view: a missed ask, waiting with Grant now.
    p.append('<g id="asks-view" opacity="0">'
             + text(16, 126, "MISSED WHILE YOU WERE AWAY", 11, MUTE, spacing=1.3)
             + text(PW - 16, 126, "KEPT 24H", 11, MUTE, anchor="end", spacing=1.3)
             + rect(14, 136, PW - 28, 84, 10, TILE, TILE_LINE)
             + text(26, 172, "12:31", 12.5, WARN)
             + text(84, 160, "RADARR_API_KEY, SONARR_API_KEY", 12, INK, 500)
             + text(84, 180, "anime-search/tools", 12, DIM)
             + text(84, 198, '"Add the 1080p release"', 12, "#d8c79c", italic=True)
             + rect(334, 165, 92, 26, 6, "#a3be8c22", "#a3be8c66") + text(380, 182, "Grant now", 12, CALM, anchor="middle")
             + text(16, 244, "Grant now takes one touch. The agent's next", 11.5, MUTE)
             + text(16, 260, "exec finds the grant instead of asking again.", 11.5, MUTE)
             + rect(14, 274, PW - 28, 36, 9, STRIP, "#353a41")
             + spans(26, 297, [("Today · answered 12 · ", FG), ("missed 1", WARN), (" · open 0", FG)], 12.5)
             + "</g>")
    # Footer
    p.append(f'<line x1="0" y1="{PH - 36}" x2="{PW}" y2="{PH - 36}" stroke="#353a41"/>'
             + spans(20, PH - 14, [("r", FG, 500), (" revoke   ", MUTE), ("R", FG, 500), (" revoke all   ", MUTE),
                                   ("1-4", FG, 500), (" tabs   ", MUTE), ("esc", FG, 500), (" close", MUTE)], 11.5))
    p.append("</g></g>")
    add("panel", "".join(p))


def build_tooltips():
    for i, s in enumerate(["JIRA_API_TOKEN granted · from pixel", "homelab · 5 secrets for 9h · one touch"]):
        w = mono_w(s, 12.5) + 24
        x = min(LAMP_X - w / 2, W - 12 - w)
        add("overlay", f'<g id="tip{i}" opacity="0">{rect(x, BAR_Y - 40, w, 28, 6, "#1d2024", "#4c566a")}'
                       + text(x + 12, BAR_Y - 21, s, 12.5, FG) + "</g>")


def build_cursor():
    add("cursor", '<g id="cursor">'
                  '<path d="M0,0 L0,22 L6,17 L10,26 L14,24 L10,15 L18,15 Z" fill="#eceff4" stroke="#1d2024" '
                  'stroke-width="1.4" stroke-linejoin="round"/></g>')


# ---------------------------------------------------------------- scenes

TERM_X = TX + 22
CMD1 = 'broker exec --request --reason "Deploying bnb.twinspear.com" \\'
CMD2 = "    CLOUDFLARE_API_TOKEN -- npm run deploy"
TERM_LINES = []  # ids of terminal lines that a scene shows, so reset can hide them


def term_line(i, t, parts):
    lid = f"tl{i}"
    add("windows", spans(TERM_X, line_y(i), parts, 13, id=lid, style="opacity:0"))
    show(lid, t, fade=0.18)
    TERM_LINES.append(lid)


def scene_desk(t0, d):
    """The desktop at rest; claude starts on a deploy."""
    term_line(2, t0 + 0.45, [("● ", ACCENT), ("Deploying bnb.twinspear.com (night look)", FG)])
    cursor.go(t0 + 0.01, *CURSOR_HOME, travel=0.01)


def scene_agent_asks(t0, d):
    """The agent runs broker exec --request and waits for a grant."""
    for i, (s, a, b) in enumerate([("● " + CMD1, 0.05, 1.15), (CMD2, 1.2, 1.6)]):
        lid = f"cmd{i}"
        typed("windows", lid, TERM_X, line_y(3 + i), s, t0 + a, t0 + b,
              parts=[("● ", ACCENT), (CMD1, FG)] if i == 0 else [(s, FG)])
        show(lid, t0 + a - 0.02, fade=0.01)
        TERM_LINES.append(lid)
    term_line(5, t0 + 1.8, [("  └ ", MUTE), ("waiting for a grant · request window open", WARN)])


def scene_request(t0, d):
    """The request window springs up; the YubiKey blinks for a touch."""
    spring_open("request", t0 + 0.1, dur=0.6, dy=10, from_scale=0.82)
    key("yubikey", t0 + 0.5, ease="cubic-bezier(.2,.8,.2,1)", opacity="0", transform="translateY(18px)")
    key("yubikey", t0 + 0.9, opacity="1", transform="translateY(0px)")


def scene_touch(t0, d):
    """A touch: the key flashes, the window says granted, the lamp lights."""
    key("yk-flash", t0 + 0.15, opacity="0", transform="scale(.8)")
    key("yk-flash", t0 + 0.2, opacity="1", transform="scale(.9)")
    key("yk-flash", t0 + 0.75, opacity="0", transform="scale(2.4)")
    swap("touch-wait", "touch-done", t0 + 0.3)
    swap("tl5", "tl5b", t0 + 0.45)
    add("windows", spans(TERM_X, line_y(5), [("  └ ", MUTE), ("granted for 1h · touched on the YubiKey", CALM)],
                         13, id="tl5b", style="opacity:0"))
    TERM_LINES.append("tl5b")
    swap("lamp-quiet", "lamp-live", t0 + 0.5)
    c = lamp_arc_len()
    key("lamp-arc", t0 + 0.5, ease="cubic-bezier(.2,.8,.2,1)", stroke_dashoffset=f"{c:.2f}")
    key("lamp-arc", t0 + 1.1, stroke_dashoffset="0")
    pop("count-1", t0 + 0.55)


def pop(el, t):
    key(el, t, ease="cubic-bezier(.2,1.6,.4,1)", opacity="0", transform="scale(.4)")
    key(el, t + 0.3, opacity="1", transform="scale(1)")


def unpop(el, t):
    key(el, t, opacity="1", transform="scale(1)")
    key(el, t + 0.2, opacity="0", transform="scale(.6)")


def scene_deploy(t0, d):
    """The window folds away and the deploy runs, the token redacted."""
    spring_close("request", t0 + 0.1, dur=0.3, dy=0, to_scale=0.9)
    key("yubikey", t0 + 0.1, opacity="1", transform="translateY(0px)")
    key("yubikey", t0 + 0.4, opacity="0", transform="translateY(18px)")
    lines = [
        [("  > ", MUTE), ("wrangler deploy", FG)],
        [("  token ", MUTE), ("[REDACTED:CLOUDFLARE_API_TOKEN]", ACCENT)],
        [("  Uploaded bnb (2.31 sec)", FG)],
        [("  Deployed bnb triggers (0.84 sec)", FG)],
        [("  https://bnb.twinspear.com", ACCENT)],
        [("● ", CALM), ("Deployed. The token never entered my context.", FG)],
    ]
    for i, parts in enumerate(lines):
        term_line(6 + i, t0 + 0.3 + i * 0.27 + (0.25 if i == 5 else 0), parts)


def scene_more_grants(t0, d):
    """Time passes; other agents get grants and the lamp counts them."""
    swap("clock-a", "clock-b", t0 + 0.1, fade=0.3)
    c = lamp_arc_len()
    key("lamp-arc", t0 + 0.1, ease="ease-in-out", stroke_dashoffset="0")
    key("lamp-arc", t0 + 1.6, stroke_dashoffset=f"{c * 0.8:.2f}")
    show("tip0", t0 + 0.15, t0 + 0.75, fade=0.15)
    unpop("count-1", t0 + 0.25)
    pop("count-2", t0 + 0.3)
    show("tip1", t0 + 0.95, t0 + 1.55, fade=0.15)
    unpop("count-2", t0 + 1.05)
    pop("count-7", t0 + 1.1)


def scene_open(t0, d):
    """A click on the lamp; the panel springs out of it."""
    cursor.click(t0 + 0.6, LAMP_X, LAMP_Y, travel=0.55)
    key("lamp-bg", t0 + 0.6, opacity="0")
    key("lamp-bg", t0 + 0.7, opacity="1")
    spring_open("panel", t0 + 0.65)
    for i, el in enumerate(["p-head", "p-tabs", "p-strip"] + [g["id"] for g in GRANTS]):
        rise(el, t0 + 0.75 + i * 0.07)
    for i, g in enumerate(GRANTS):
        c = 2 * math.pi * 18
        frac = g["ring"][0]
        key(g["id"] + "-ring", t0 + 0.95 + i * 0.07, ease="cubic-bezier(.2,.8,.2,1)", stroke_dashoffset=f"{c:.2f}")
        key(g["id"] + "-ring", t0 + 1.7 + i * 0.07, stroke_dashoffset=f"{c * (1 - frac):.2f}")
        ln = spark_lengths[g["id"]]
        key(g["id"] + "-spark", t0 + 1.0 + i * 0.07, ease="ease-out", stroke_dashoffset=f"{ln:.0f}")
        key(g["id"] + "-spark", t0 + 1.8 + i * 0.07, stroke_dashoffset="0")


def tab_center(i):
    return PX + TAB_X0 + TAB_W * (i + 0.5), PY + 82


def scene_asks(t0, d):
    """A look at Asks: the ask that was missed at lunch, ready for Grant now."""
    x, y = tab_center(1)
    cursor.click(t0 + 0.5, x, y, travel=0.45)
    t = t0 + 0.5
    key("tab-pill", t, ease="cubic-bezier(.3,1.3,.5,1)", transform="translateX(0px)")
    key("tab-pill", t + 0.3, transform=f"translateX({TAB_W}px)")
    key("tab-live", t + 0.05, fill=DARK)
    key("tab-live", t + 0.15, fill=DIM)
    key("tab-asks", t + 0.1, fill=DIM)
    key("tab-asks", t + 0.25, fill=DARK)
    key("live-view", t, ease="cubic-bezier(.4,0,.2,1)", opacity="1", transform="translateX(0px)")
    key("live-view", t + 0.28, opacity="0", transform="translateX(-36px)")
    key("asks-view", t + 0.1, ease="cubic-bezier(.2,.8,.2,1)", opacity="0", transform="translateX(36px)")
    key("asks-view", t + 0.42, opacity="1", transform="translateX(0px)")


def scene_revoke(t0, d):
    """Back to Live, then Revoke all: every grant drops and the lamp goes gray."""
    x, y = tab_center(0)
    cursor.click(t0 + 0.45, x, y, travel=0.45)
    t = t0 + 0.45
    key("tab-pill", t, ease="cubic-bezier(.3,1.3,.5,1)", transform=f"translateX({TAB_W}px)")
    key("tab-pill", t + 0.3, transform="translateX(0px)")
    key("tab-asks", t + 0.05, fill=DARK)
    key("tab-asks", t + 0.15, fill=DIM)
    key("tab-live", t + 0.1, fill=DIM)
    key("tab-live", t + 0.25, fill=DARK)
    key("asks-view", t, ease="cubic-bezier(.4,0,.2,1)", opacity="1", transform="translateX(0px)")
    key("asks-view", t + 0.28, opacity="0", transform="translateX(36px)")
    key("live-view", t + 0.1, ease="cubic-bezier(.2,.8,.2,1)", opacity="0", transform="translateX(-36px)")
    key("live-view", t + 0.42, opacity="1", transform="translateX(0px)")

    r = t0 + 1.2
    cursor.click(r, PX + 376, PY + 130, travel=0.5)
    key("revoke-all-btn", r - 0.05, stroke="#474d57")
    key("revoke-all-btn", r + 0.05, stroke=ACT)
    key("revoke-all-btn", r + 0.5, stroke="#474d57")
    for i, g in enumerate(GRANTS):
        s = r + 0.12 + i * 0.1
        key(g["id"], s, ease="cubic-bezier(.5,0,.75,0)", transform="translateX(0px)", opacity="1")
        key(g["id"], s + 0.35, transform="translateX(44px)", opacity="0")
    key("p-strip", r + 0.45, opacity="1")
    key("p-strip", r + 0.65, opacity="0")
    show("live-empty", r + 0.75, fade=0.3)
    swap("chip-live", "chip-none", r + 0.6)
    swap("tab-live", "tab-live0", r + 0.6)
    key("tab-live0", r + 0.6, fill=DARK)
    swap("lamp-live", "lamp-quiet", r + 0.6)
    unpop("count-7", r + 0.6)


def scene_close(t0, d):
    """A click on the lamp folds the panel back into it."""
    cursor.click(t0 + 0.5, LAMP_X, LAMP_Y, travel=0.5)
    spring_close("panel", t0 + 0.55)
    key("lamp-bg", t0 + 0.55, opacity="1")
    key("lamp-bg", t0 + 0.75, opacity="0")


def scene_reset(t0, d):
    """Back to the first frame: terminal cleared, clock back, panel reset."""
    for lid in TERM_LINES:
        key(lid, t0, opacity="1" if lid != "tl5" else "0")
        key(lid, t0 + 0.3, opacity="0")
    for i in range(2):
        key(f"cmd{i}-cover", t0 + 0.35, transform=f"translateX({mono_w([CMD1, CMD2][i], 13) + 10:.0f}px)")
        key(f"cmd{i}-cover", t0 + 0.36, transform="translateX(0px)")
    swap("clock-b", "clock-a", t0 + 0.05, fade=0.3)
    # Everything else that ended somewhere other than where it started
    # (the panel's insides, the request window, rings, ripples) is hidden
    # by now, so it snaps back to its first frame unseen.
    s = t0 + 0.4
    for el in list(tracks):
        if el == "cursor" or max(k[0] for k in tracks[el]) > s:
            continue
        fr = frames(el)
        if fr[0][1] != fr[-1][1]:
            key(el, s, **state_at(el, s))
            key(el, s + 0.01, **fr[0][1])
    cursor.go(t0 + d - 0.02, *CURSOR_HOME, travel=0.6)


SCENES = [
    ("desk", 1.0, scene_desk),
    ("agent-asks", 2.2, scene_agent_asks),
    ("request", 2.6, scene_request),
    ("touch", 1.2, scene_touch),
    ("deploy", 2.4, scene_deploy),
    ("more-grants", 1.8, scene_more_grants),
    ("open", 2.6, scene_open),
    ("asks", 2.0, scene_asks),
    ("revoke", 2.6, scene_revoke),
    ("close", 1.4, scene_close),
    ("reset", 0.8, scene_reset),
]

STARTS, T = {}, 0.0
for _name, _sec, _fn in SCENES:
    STARTS[_name] = T
    T += _sec


def start(name):
    return STARTS[name]


# The reduced-motion still: the panel open on Live, every tile in place.
STILL = start("open") + 2.5

# ---------------------------------------------------------------- compile


def frames(el):
    """el's keys sorted, merged by time, every frame carrying every property."""
    ks = sorted(tracks[el], key=lambda k: k[0])
    merged = []
    for t, props, ease in ks:
        t = min(max(t, 0.0), T)
        if merged and abs(merged[-1][0] - t) < 1e-6:
            merged[-1][1].update(props)
            merged[-1] = (merged[-1][0], merged[-1][1], ease or merged[-1][2])
        else:
            merged.append((t, dict(props), ease))
    names = []
    for _, p, _ in merged:
        names += [n for n in p if n not in names]
    first = {}
    for n in names:
        first[n] = next(p[n] for _, p, _ in merged if n in p)
    cur, out = dict(first), []
    for t, p, ease in merged:
        cur.update(p)
        out.append((t, dict(cur), ease))
    if out[0][0] > 0:
        out.insert(0, (0.0, dict(out[0][1]), None))
    if out[-1][0] < T:
        out.append((T, dict(out[-1][1]), None))
    return out


def state_at(el, t):
    """el's full set of properties at time t, from its keys so far."""
    state = None
    for ft, props, _ in frames(el):
        if ft <= t + 1e-6:
            state = props
    return dict(state)


def decl(props):
    return ";".join(f"{k}:{v}" for k, v in props.items())


def compile_css():
    css = []
    for el in tracks:
        fr = frames(el)
        if fr[0][1] != fr[-1][1]:
            print(f"warning: {el} ends the loop at {decl(fr[-1][1])} but starts at {decl(fr[0][1])}")
        css.append(f"#{el}{{animation:k-{el} {T:.2f}s linear infinite both}}")
        body = []
        for t, props, ease in fr:
            e = f";animation-timing-function:{ease}" if ease else ""
            body.append(f"{t / T * 100:.3f}%{{{decl(props)}{e}}}")
        css.append(f"@keyframes k-{el}{{{''.join(body)}}}")
    return "\n".join(css)


def still_css(t):
    """Every object frozen at time t."""
    css = []
    for el in tracks:
        css.append(f"#{el}{{animation:none!important;{decl(state_at(el, t))}}}")
    css.append(".pulse,.blink{animation:none!important}")
    return "\n".join(css)


BASE_CSS = """
text{white-space:pre}
.pulse{animation:pulse 1.4s ease-out infinite}
@keyframes pulse{0%{opacity:.7;transform:scale(.6)}80%,100%{opacity:0;transform:scale(1.7)}}
.blink{animation:blink 1.1s ease-in-out infinite}
@keyframes blink{50%{opacity:.35}}
"""


def render(still=None):
    build_desk()
    build_terminal()
    build_request()
    build_yubikey()
    build_bar()
    build_panel()
    build_tooltips()
    build_cursor()
    for name, sec, fn in SCENES:
        fn(STARTS[name], sec)

    css = BASE_CSS + compile_css()
    if still is not None:
        css += "\n" + still_css(still)
    css += ("\n@media (prefers-reduced-motion: reduce){\n" + still_css(STILL)
            + "\n#cursor,[id^=ripple],#tip0,#tip1{display:none}\n}")

    label = ("An animated loop in two acts. First credential-broker: in a terminal, claude runs broker exec "
             "--request for CLOUDFLARE_API_TOKEN, broker's request window springs up asking for a YubiKey touch, "
             "the touch grants it for an hour, and the deploy runs with the token shown as "
             "[REDACTED:CLOUDFLARE_API_TOKEN]. Then Signal Box: the lamp in waybar counts grants as they arrive, "
             "a click opens the panel out of that icon with three live grants and their time left, a look at a "
             "missed ask on the Asks tab, then Revoke all drops every grant, the lamp goes gray, and the panel "
             "folds back into the lamp.")
    head = (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" '
            f'font-family="{FONT}" role="img" aria-label="{esc(label)}">'
            "<title>credential-broker and Signal Box</title>"
            f"<desc>{esc(label)}</desc>"
            f"<style>{css}</style>"
            f"""<defs>
<linearGradient id="wall" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="{BG0}"/><stop offset="1" stop-color="{BG1}"/></linearGradient>
<radialGradient id="haze1" cx=".85" cy="0" r=".75"><stop offset="0" stop-color="{ACCENT}" stop-opacity=".13"/><stop offset="1" stop-color="{ACCENT}" stop-opacity="0"/></radialGradient>
<radialGradient id="haze2" cx=".05" cy="1" r=".6"><stop offset="0" stop-color="#b48ead" stop-opacity=".08"/><stop offset="1" stop-color="#b48ead" stop-opacity="0"/></radialGradient>
<filter id="shadow" x="-20%" y="-20%" width="140%" height="150%"><feDropShadow dx="0" dy="18" stdDeviation="22" flood-color="#000" flood-opacity=".6"/></filter>
<clipPath id="screen"><rect width="{W}" height="{H}" rx="18"/></clipPath>
<clipPath id="term-clip"><rect x="{TX + 1}" y="{TY + 30}" width="{TW - 2}" height="{TH - 31}"/></clipPath>
</defs>""")
    body = "".join("".join(layers[name]) for name in LAYERS)
    return head + f'<g clip-path="url(#screen)">{body}</g></svg>\n'


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--still", type=float, help="freeze every object at this many seconds into the loop")
    ap.add_argument("--out", type=Path, default=Path(__file__).resolve().parent.parent / "hero.svg")
    a = ap.parse_args()
    svg = render(a.still)
    a.out.write_text(svg)
    scenes = ", ".join(f"{n} {s:g}s" for n, s, _ in SCENES)
    print(f"wrote {a.out} ({len(svg.encode()) // 1024} KB, {T:g}s loop: {scenes})")


if __name__ == "__main__":
    main()

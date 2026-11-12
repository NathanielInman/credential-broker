#!/usr/bin/env python3
"""Writes artwork/hero.svg: the README's animated hero.

One loop in two acts, on the desktop the siblings' heroes use, under the
floating Platform waybar. First broker: an agent in T3 Code asks for
CLOUDFLARE_API_TOKEN, the request window springs up while the Signal Box lamp
turns to an amber key, a YubiKey touch grants it, and the deploy runs with
the token redacted from its output. Then Signal Box: the lamp's pie drains
and its count climbs as grants arrive, a click opens the panel out of the
lamp on Live, a look at the Register tab (the deploy is there), Revoke all,
and the panel folds back into the lamp.

Pure CSS animation inside the SVG, so GitHub plays it in an <img>. With
reduced motion the art is a still of the open panel.

    python3 artwork/hero/hero.py                 # write artwork/hero.svg
    python3 artwork/hero/hero.py --still 13.5    # a frozen frame, for checking
    python3 artwork/hero/hero.py --still 13.5 --out /tmp/frame.svg

How it is put together
----------------------
Objects that live across scenes (the desk, the T3 Code window, the request
window, the bar and its lamp, the panel, the cursor) are drawn once by the
build_* functions, each animated part with an id. Nothing in them moves by
itself.

The Signal Box panel and the request window are drawn by sb_panel() and
request_window(), which know nothing about the desk or the bar: each draws
its window in its own coordinates at its real pixel size, for a state the
story asks for. The README hero of the dot-files repo ports them.

SCENES is the timeline: (name, seconds, function). The loop length is the
sum of the seconds, and start(name) is where a scene begins. Each scene
function gets its own start time t0 and length d and adds keyframes to the
objects with key(), cut(), show() and friends, using absolute times
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
import re
from collections import defaultdict
from pathlib import Path

W, H = 1200, 700

# The desk, in Nord as in Roundhouse's and Switchyard's heroes.
BG0, BG1 = "#15181d", "#0d0f12"
INK, FG, DIM, MUTE = "#e8ebf0", "#c5c8c6", "#8d949f", "#666d78"
ACCENT, CALM, WARN, ACT, DARK = "#88c0d0", "#a3be8c", "#ebcb8b", "#bf616a", "#1d2024"
TERM_BG, TERM_TOP = "#1a1d22", "#1f2328"

# The Platform waybar (dot-files packages/waybar/.config/waybar/style.css):
# frosted pills on a transparent bar, off is dim, on is a color.
PILL, PILL_LINE = "rgba(40,42,46,.84)", "rgba(76,86,106,.55)"
HOVER, WS_ON = "rgba(129,161,193,.18)", "rgba(129,161,193,.26)"
B_FG, B_BRIGHT, B_DIM = "#c5c8c6", "#d8dee9", "#616e88"
B_FROST, B_GREEN, B_YELLOW, B_PURPLE = "#88c0d0", "#a3be8c", "#ebcb8b", "#b48ead"
B_ASK_LOW = "#6b5d3f"  # where the asking key's pulse dims to (@keyframes broker-ask)

# The request window, in broker-dialog's own colors (broker-dialog/ui.html).
DLG_BG, DLG_FG, DLG_MUTED, DLG_FAINT = "#1b1c21", "#e9e9ee", "#9a9ca8", "#7d808c"
DLG_PANEL, DLG_LINE, DLG_BUTTON, DLG_PRIMARY = "#24262d", "#30323b", "#33353e", "#4f7ef7"
CLAIM_BG, CLAIM_LINE, CLAIM_FG = "#2a261e", "#c9a24a", "#d9b45c"
TOUCH_BG, TOUCH_LINE, OK = "#1d2a22", "#2f4c37", "#5fd38a"

FONT = "JetBrains Mono, DejaVu Sans Mono, Menlo, Consolas, monospace"
SANS = "Inter, Noto Sans, Cantarell, DejaVu Sans, Helvetica Neue, Arial, sans-serif"

# The bar: 34px tall, 6px off the bottom and the sides, 6px between pills.
BAR_H = 34
BAR_Y = H - 6 - BAR_H
BAR_MID = BAR_Y + BAR_H / 2
BAR_BASE = BAR_MID + 4.7  # text baseline in the bar
BAR_SIZE, CW = 13, 7.8    # bar text size and its character width
ICON = 14                 # the slot a bar glyph takes

TX, TY, TW, TH = 36, 34, 664, 372           # the T3 Code window
RW, RH, RH_DONE = 470, 505, 435             # request window, waiting and granted
RX, RY = (W - RW) // 2, 78
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
         italic=False, id=None, cls=None, style=None, extra=""):
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
    return f'<text x="{x:g}" y="{y:g}"{a}{attrs(id, cls, style, extra)}>{esc(s)}</text>'


def spans(x, y, parts, size=13, family=None, id=None, style=None):
    """A line in several looks: parts are (text, fill), (text, fill, weight)
    or (text, fill, weight, italic)."""
    fam = f' font-family="{family}"' if family else ""
    inner = ""
    for p in parts:
        s, fill = p[0], p[1]
        w = f' font-weight="{p[2]}"' if len(p) > 2 and p[2] != 400 else ""
        it = ' font-style="italic"' if len(p) > 3 and p[3] else ""
        inner += f'<tspan fill="{fill}"{w}{it}>{esc(s)}</tspan>'
    return f'<text x="{x:g}" y="{y:g}" font-size="{size}"{fam}{attrs(id, None, style)}>{inner}</text>'


def rect(x, y, w, h, rx=0, fill="none", stroke=None, id=None, cls=None, style=None, extra=""):
    s = f' stroke="{stroke}"' if stroke else ""
    return f'<rect x="{x:g}" y="{y:g}" width="{w:g}" height="{h:g}" rx="{rx:g}" fill="{fill}"{s}{attrs(id, cls, style, extra)}/>'


def mono_w(s, size):
    """Width of s in the monospace stack (0.6em a glyph)."""
    return len(s) * size * 0.6


# Nerd Font glyphs the bar shows, as outlines from JetBrainsMono Nerd Font
# (font units, y up, with each one's bounding box): an <img> loads no fonts,
# so the bar's icons travel as paths.
GLYPHS = {
    "kitty": ((0, -44, 1038, 764), "M17 665Q0 682 0 706Q0 730 17 747Q34 764 58 764Q82 764 99 747L445 400Q462 384 462 360Q462 336 445 319L99 -27Q82 -44 58 -44Q34 -44 17 -27Q0 -10 0 14Q0 38 17 54L322 360ZM462 71H981Q1006 70 1022 54Q1038 38 1038 14Q1038 -10 1022 -27Q1006 -44 981 -44H462Q438 -44 421 -27Q404 -10 404 14Q404 38 421 54Q438 70 462 71Z"),
    "neovide": ((0, -48, 873, 768), "M13 768 334 762 354 738V682L337 664H312Q307 664 304 662Q302 658 302 654L299 410V408Q301 406 301 404Q303 400 304 400Q308 398 310 398Q312 398 318 402L569 648Q569 648 570 648Q572 650 572 654Q573 656 573 658Q573 660 570 662Q568 664 563 664H541L526 684V742L543 764L858 762L873 746V680L156 -48H76L53 -24L51 656Q51 660 48 662Q46 666 42 666L16 668L0 682V752Z"),
    "chrome": ((0, -102, 923, 821), "M0 360Q0 482 62 592L260 247Q290 193 344 161Q398 129 462 129Q498 129 535 141L398 -98Q228 -74 114 56Q0 186 0 360ZM658 241Q692 295 692 360Q692 465 614 533H890Q923 454 923 360Q922 168 788 34Q654 -100 462 -102ZM861 590H462Q378 590 314 538Q250 486 235 405L98 644Q162 726 258 774Q354 822 462 821Q588 820 694 758Q800 696 861 590ZM303 360Q304 426 350 472Q396 518 462 518Q528 518 574 472Q620 426 620 360Q620 294 574 248Q528 202 462 201Q396 202 350 248Q304 294 303 360Z"),
    "slack": ((0, -44, 808, 764), "M170 253Q170 218 145 193Q120 168 85 168Q50 168 25 193Q0 218 0 253Q0 288 25 313Q50 338 85 338H170ZM212 253Q212 288 237 313Q262 338 297 338Q332 338 357 313Q382 288 382 253V41Q382 6 357 -19Q332 -44 297 -44Q262 -44 237 -19Q212 6 212 41ZM297 594Q262 594 237 619Q212 644 212 679Q212 714 237 739Q262 764 297 764Q332 764 357 739Q382 714 382 679V594ZM297 551Q332 551 357 526Q382 501 382 466Q382 431 357 406Q332 381 297 381H85Q50 381 25 406Q0 431 0 466Q0 501 25 526Q50 551 85 551ZM638 466Q638 501 663 526Q688 551 723 551Q758 551 783 526Q808 501 808 466Q808 431 783 406Q758 381 723 381H638ZM595 466Q595 431 570 406Q545 381 510 381Q474 381 450 406Q426 431 426 466V679Q426 714 450 739Q474 764 510 764Q545 764 570 739Q595 714 595 679ZM510 126Q545 126 570 101Q595 76 595 41Q595 6 570 -19Q545 -44 510 -44Q474 -44 450 -19Q426 6 426 41V126ZM510 168Q474 168 450 193Q426 218 426 253Q426 288 450 313Q474 338 510 338H723Q758 338 783 313Q808 288 808 253Q808 218 783 193Q758 168 723 168Z"),
    "t3": ((0, -102, 1154, 822), "M577 822Q601 820 618 804Q635 788 635 764V648H851Q905 648 943 610Q981 572 981 518V28Q981 -26 943 -64Q905 -102 851 -102H303Q249 -102 211 -64Q173 -26 173 28V518Q173 572 211 610Q249 648 303 648H519V764Q519 788 536 804Q553 820 577 822ZM375 130Q362 128 354 120Q346 112 346 100Q346 88 354 80Q362 72 375 72H433Q446 72 454 80Q462 88 462 100Q462 112 454 120Q446 128 433 130ZM548 130Q536 128 528 120Q520 112 519 100Q520 88 528 80Q536 72 548 72H606Q618 72 626 80Q634 88 635 100Q634 112 626 120Q618 128 606 130ZM721 130Q708 128 700 120Q692 112 692 100Q692 88 700 80Q708 72 721 72H779Q792 72 800 80Q808 88 808 100Q808 112 800 120Q792 128 779 130ZM455 309Q476 330 476 360Q476 390 455 411Q434 432 404 432Q374 432 353 411Q332 390 332 360Q332 330 353 309Q374 288 404 288Q434 288 455 309ZM699 309Q720 288 750 288Q780 288 801 309Q822 330 822 360Q822 390 801 411Q780 432 750 432Q720 432 699 411Q678 390 678 360Q678 330 699 309ZM87 418H115V72H87Q52 70 26 96Q0 122 0 158V332Q0 368 26 393Q52 418 87 418ZM1067 418Q1102 418 1128 393Q1154 368 1154 332V158Q1154 122 1128 96Q1102 70 1067 72H1038V418Z"),
    "roundhouse": ((0, -56, 832, 776), "M416 776Q334 776 257 745Q180 714 121 655Q62 596 31 519Q0 442 0 360Q0 278 31 201Q62 124 121 65Q180 6 257 -25Q334 -56 416 -56Q498 -56 575 -25Q652 6 711 65Q770 124 801 201Q832 278 832 360Q832 442 801 519Q770 596 711 655Q652 714 575 745Q498 776 416 776Z"),
    "switchyard": ((0, -78, 875, 798), "M770 380H707V548Q707 580 682 605Q658 630 625 630H457V692Q457 736 426 767Q396 798 354 798Q311 798 280 766Q250 734 250 692V630H82Q49 630 24 605Q0 580 0 548V390H62Q109 390 142 356Q174 322 174 276Q174 230 142 196Q109 162 62 162H0V4Q0 -28 24 -53Q49 -78 82 -78H240V-16Q240 32 274 64Q307 96 354 96Q400 96 434 64Q467 32 467 -16V-78H625Q658 -78 682 -53Q707 -28 707 4V172H770Q813 172 844 203Q875 234 875 276Q875 318 844 349Q813 380 770 380Z"),
    "broker_quiet": ((0, -56, 832, 776), "M416 26Q326 26 249 71Q172 116 127 193Q82 270 82 360Q82 450 127 527Q172 604 249 649Q326 694 416 694Q506 694 583 649Q660 604 705 527Q750 450 750 360Q750 270 705 193Q660 116 583 71Q506 26 416 26ZM416 776Q334 776 257 745Q180 714 121 655Q62 596 31 519Q0 442 0 360Q0 278 31 201Q62 124 121 65Q180 6 257 -25Q334 -56 416 -56Q498 -56 575 -25Q652 6 711 65Q770 124 801 201Q832 278 832 360Q832 442 801 519Q770 596 711 655Q652 714 575 745Q498 776 416 776Z"),
    "broker_key": ((0, 110, 918, 610), "M250 276Q217 276 192 300Q168 325 168 360Q168 395 192 420Q217 444 251 444Q285 444 310 420Q334 395 334 360Q334 325 310 300Q285 276 250 276ZM484 444Q459 516 394 563Q330 610 250 610Q184 610 126 576Q68 542 34 485Q0 428 0 360Q0 292 34 235Q68 178 126 144Q184 110 250 110Q330 110 394 157Q459 204 484 276H625V110H793V276H918V444Z"),
    "slice1": ((0, -56, 832, 776), "M416 776Q529 776 625 720Q721 665 776 569Q832 473 832 360Q832 247 776 151Q721 55 625 0Q529 -56 416 -56Q303 -56 207 0Q111 55 56 151Q0 247 0 360Q0 473 56 569Q111 665 207 720Q303 776 416 776ZM416 694Q326 694 249 649Q172 604 127 527Q82 450 82 360Q82 270 127 193Q172 116 249 71Q326 26 416 26Q506 26 583 71Q660 116 705 193Q750 270 750 360Q750 450 705 527Q660 604 583 649Q506 694 416 694ZM416 651Q475 651 528 628Q582 606 623 567L416 360Z"),
    "slice2": ((0, -56, 832, 776), "M416 776Q529 776 625 720Q721 665 776 569Q832 473 832 360Q832 247 776 151Q721 55 625 0Q529 -56 416 -56Q303 -56 207 0Q111 55 56 151Q0 247 0 360Q0 473 56 569Q111 665 207 720Q303 776 416 776ZM416 694Q326 694 249 649Q172 604 127 527Q82 450 82 360Q82 270 127 193Q172 116 249 71Q326 26 416 26Q506 26 583 71Q660 116 705 193Q750 270 750 360Q750 450 705 527Q660 604 583 649Q506 694 416 694ZM416 651Q496 651 562 612Q629 573 668 506Q707 440 707 360H416Z"),
    "slice3": ((0, -56, 832, 776), "M416 776Q529 776 625 720Q721 665 776 569Q832 473 832 360Q832 247 776 151Q721 55 625 0Q529 -56 416 -56Q303 -56 207 0Q111 55 56 151Q0 247 0 360Q0 473 56 569Q111 665 207 720Q303 776 416 776ZM416 694Q326 694 249 649Q172 604 127 527Q82 450 82 360Q82 270 127 193Q172 116 249 71Q326 26 416 26Q506 26 583 71Q660 116 705 193Q750 270 750 360Q750 450 705 527Q660 604 583 649Q506 694 416 694ZM416 651Q496 651 562 612Q629 573 668 506Q707 440 707 360Q707 301 684 248Q662 194 623 153L416 360Z"),
    "slice4": ((0, -56, 832, 776), "M416 776Q529 776 625 720Q721 665 776 569Q832 473 832 360Q832 247 776 151Q721 55 625 0Q529 -56 416 -56Q303 -56 207 0Q111 55 56 151Q0 247 0 360Q0 473 56 569Q111 665 207 720Q303 776 416 776ZM416 694Q326 694 249 649Q172 604 127 527Q82 450 82 360Q82 270 127 193Q172 116 249 71Q326 26 416 26Q506 26 583 71Q660 116 705 193Q750 270 750 360Q750 450 705 527Q660 604 583 649Q506 694 416 694ZM416 651Q496 651 562 612Q629 573 668 506Q707 440 707 360Q707 280 668 214Q629 147 562 108Q496 69 416 69Z"),
    "slice5": ((0, -56, 832, 776), "M416 776Q529 776 625 720Q721 665 776 569Q832 473 832 360Q832 247 776 151Q721 55 625 0Q529 -56 416 -56Q303 -56 207 0Q111 55 56 151Q0 247 0 360Q0 473 56 569Q111 665 207 720Q303 776 416 776ZM416 694Q326 694 249 649Q172 604 127 527Q82 450 82 360Q82 270 127 193Q172 116 249 71Q326 26 416 26Q506 26 583 71Q660 116 705 193Q750 270 750 360Q750 450 705 527Q660 604 583 649Q506 694 416 694ZM416 651Q496 651 562 612Q629 573 668 506Q707 440 707 360Q707 280 668 214Q629 147 562 108Q496 69 416 69Q357 69 304 92Q250 114 209 153L416 360Z"),
    "slice6": ((0, -56, 832, 776), "M416 776Q529 776 625 720Q721 665 776 569Q832 473 832 360Q832 247 776 151Q721 55 625 0Q529 -56 416 -56Q303 -56 207 0Q111 55 56 151Q0 247 0 360Q0 473 56 569Q111 665 207 720Q303 776 416 776ZM416 694Q326 694 249 649Q172 604 127 527Q82 450 82 360Q82 270 127 193Q172 116 249 71Q326 26 416 26Q506 26 583 71Q660 116 705 193Q750 270 750 360Q750 450 705 527Q660 604 583 649Q506 694 416 694ZM416 651Q496 651 562 612Q629 573 668 506Q707 440 707 360Q707 280 668 214Q629 147 562 108Q496 69 416 69Q336 69 270 108Q203 147 164 214Q125 280 125 360H416Z"),
    "slice7": ((0, -56, 832, 776), "M416 776Q529 776 625 720Q721 665 776 569Q832 473 832 360Q832 247 776 151Q721 55 625 0Q529 -56 416 -56Q303 -56 207 0Q111 55 56 151Q0 247 0 360Q0 473 56 569Q111 665 207 720Q303 776 416 776ZM416 694Q326 694 249 649Q172 604 127 527Q82 450 82 360Q82 270 127 193Q172 116 249 71Q326 26 416 26Q506 26 583 71Q660 116 705 193Q750 270 750 360Q750 450 705 527Q660 604 583 649Q506 694 416 694ZM416 651Q496 651 562 612Q629 573 668 506Q707 440 707 360Q707 280 668 214Q629 147 562 108Q496 69 416 69Q336 69 270 108Q203 147 164 214Q125 280 125 360Q125 419 148 472Q170 526 209 567L416 360Z"),
    "slice8": ((0, -56, 832, 776), "M416 651Q496 651 562 612Q629 573 668 506Q707 440 707 360Q707 280 668 214Q629 147 562 108Q496 69 416 69Q336 69 270 108Q203 147 164 214Q125 280 125 360Q125 440 164 506Q203 573 270 612Q336 651 416 651ZM416 776Q529 776 625 720Q721 665 776 569Q832 473 832 360Q832 247 776 151Q721 55 625 0Q529 -56 416 -56Q303 -56 207 0Q111 55 56 151Q0 247 0 360Q0 473 56 569Q111 665 207 720Q303 776 416 776ZM416 694Q326 694 249 649Q172 604 127 527Q82 450 82 360Q82 270 127 193Q172 116 249 71Q326 26 416 26Q506 26 583 71Q660 116 705 193Q750 270 750 360Q750 450 705 527Q660 604 583 649Q506 694 416 694Z"),
    "vpn_off": ((0, -99, 918, 819), "M125 819Q92 819 68 794Q43 770 43 735V569Q43 534 68 510Q92 485 125 485H0V401H500V485H375Q410 485 434 510Q459 534 459 569V735Q459 770 434 794Q410 819 375 819ZM125 735H375V569H125ZM543 319Q508 319 484 294Q459 270 459 235V69Q459 34 484 10Q508 -15 543 -15H418V-99H918V-15H793Q826 -15 850 10Q875 34 875 69V235Q875 270 850 294Q826 319 793 319ZM121 299 61 241 150 151 61 63 121 5 209 92 297 5 357 63 268 151 357 241 297 299 209 210ZM543 235H793V69H543Z"),
    "bell": ((1, -102, 811, 822), "M406 822Q430 820 447 804Q464 788 464 764V730Q564 710 629 630Q694 550 694 446V412Q694 282 782 182L795 168Q820 140 805 106Q790 72 752 72H60Q22 72 7 106Q-8 140 17 168L30 182Q117 280 117 412V446Q116 550 182 630Q248 710 348 730V764Q348 788 365 804Q382 820 406 822ZM488 -68Q454 -102 406 -102Q358 -102 324 -68Q290 -34 290 14H406H521Q521 -34 488 -68Z"),
    "volume": ((0, -52, 1154, 772), "M962 763Q1052 690 1103 585Q1154 480 1154 360Q1154 240 1103 135Q1052 30 962 -43Q948 -54 930 -52Q912 -50 901 -37Q890 -23 892 -5Q894 13 907 24Q982 84 1024 172Q1066 260 1067 360Q1066 460 1024 548Q982 636 907 696Q894 707 892 725Q890 743 901 757Q912 770 930 772Q948 774 962 763ZM853 628Q913 580 947 510Q981 440 981 360Q981 280 947 210Q913 140 853 91Q839 80 821 82Q803 84 792 97Q780 111 782 129Q784 147 798 158Q894 236 894 360Q894 483 798 561Q784 572 782 590Q780 608 792 622Q803 636 821 638Q839 640 853 629ZM744 494Q808 441 808 360Q808 278 744 225Q730 214 712 216Q694 218 683 232Q672 246 674 264Q676 282 689 293Q721 320 721 360Q721 400 689 427Q676 438 674 456Q672 474 683 488Q694 502 712 504Q730 506 744 494ZM543 758Q577 743 577 706V14Q577 -24 543 -39Q508 -54 481 -30L238 187H115Q68 186 34 220Q0 254 0 302V417Q0 465 34 499Q68 533 115 533H238L481 749Q508 773 543 758Z"),
    "mouse": ((0, -97, 668, 817), "M293 817Q211 807 144 760Q78 714 39 642Q0 571 0 487H293ZM0 237Q0 147 45 70Q90 -7 167 -52Q244 -97 334 -97Q424 -97 501 -52Q578 -7 623 70Q668 147 668 237V403H0ZM375 817V487H668Q668 571 629 642Q590 714 524 760Q457 807 375 817Z"),
    "mic": ((9, -36, 591, 756), "M300 756Q353 756 389 720Q425 684 425 630V380Q425 330 389 293Q353 256 300 256Q247 256 211 293Q175 330 175 380V630Q175 684 211 720Q247 756 300 756ZM591 380Q591 308 559 246Q527 184 469 144Q411 104 341 92V-36H259V92Q189 104 131 144Q73 184 41 246Q9 308 9 380H91Q91 340 108 302Q124 264 154 234Q183 204 221 189Q259 174 300 174Q357 174 404 201Q452 228 480 276Q509 324 509 380Z"),
    "chevron": ((69, -44, 531, 764), "M86 400V401L432 747Q449 764 473 764Q497 764 514 747Q531 730 531 706Q531 682 514 665L208 360L514 54Q531 38 531 14Q531 -10 514 -27Q497 -44 473 -44Q449 -44 432 -27L86 319Q69 336 69 360Q69 384 86 400Z"),
    "discord": ((0, -120, 832, 840), "M832 -120 613 90 641 6Q642 6 104 6Q61 6 30 36Q0 66 0 110V734Q0 778 31 809Q62 840 104 840H728Q771 840 802 808Q832 776 832 734ZM416 598Q367 598 316 586Q281 576 250 562L227 548Q252 574 301 594L344 610L336 616Q293 616 244 592Q219 580 203 566Q170 500 154 428Q140 376 137 328L135 288Q172 242 230 224Q259 216 281 218L311 256Q288 262 266 276Q250 288 236 300L225 312L250 298Q281 282 316 274Q365 260 416 260Q467 260 516 274Q551 284 582 298L607 312L596 300Q582 286 566 276Q545 262 521 256L551 218L576 220Q605 224 633 236Q670 254 697 288L695 328Q691 376 678 428Q662 500 629 566L609 580Q586 594 561 604Q528 618 496 616L488 610L531 594Q580 574 605 548L582 562Q551 576 516 586Q465 598 416 598ZM330 440Q350 440 364 424Q379 408 379 386Q379 364 364 349Q350 334 330 334Q311 334 296 349Q281 364 281 386Q281 408 295 424Q309 440 330 440ZM504 440Q524 440 538 424Q553 408 553 386Q553 364 538 349Q523 334 504 334Q484 334 470 349Q455 364 455 386Q455 408 469 424Q483 440 504 440Z"),
    "bluetooth": ((0, -102, 692, 821), "M470 513 392 435V590ZM392 116V272L469 194ZM692 354Q692 244 676 164Q660 84 634 33Q606 -18 563 -48Q520 -78 472 -90Q424 -102 358 -102Q293 -102 242 -90Q190 -78 144 -48Q98 -17 67 33Q36 83 18 164Q0 245 0 354Q0 489 26 583Q52 678 102 728Q152 778 212 800Q273 822 354 821Q436 822 494 800Q552 778 598 727Q644 676 668 582Q692 488 692 354ZM406 354 549 513 324 755V436L191 570L142 521L309 353L142 186L191 137L324 270L328 -36L558 194Z"),
}


def glyph(name, cx, cy, fill, size=ICON, id=None, cls=None, style=None):
    """A Nerd Font glyph centered on (cx, cy), size px to the em."""
    (x0, y0, x1, y1), d = GLYPHS[name]
    s = size / 1000
    tx, ty = cx - (x0 + x1) / 2 * s, cy + (y0 + y1) / 2 * s
    return (f'<path d="{d}" fill="{fill}" transform="translate({tx:.2f},{ty:.2f}) scale({s:.4f},{-s:.4f})"'
            f'{attrs(id, cls, style)}/>')


# ---------------------------------------------------------------- Signal Box
#
# The panel (broker-board/ui.html), drawn in its own coordinates: 460 wide,
# min(640, the monitor's free height - 20) tall, its top-left at (0, 0).
# Positions come from the page laid out in a browser with the snapshot
# below, in the colors the panel takes from the Platform waybar's
# style.css (theme.go: panel and tiles lift the bar color, dim mixes fg
# into it, the border is the first border on the stylesheet).
#
#   sb_panel(state, h)         the whole panel for one state
#   sb_frame(h)                its background and shadow (needs SB_DEFS)
#   sb_chrome(state, h, stag)  header, tabs and footer
#   sb_body(state, h, stag, clip)  the tab's content, clipped to <main>
#
# States: "live" (7 secrets in 3 grants, the first row selected),
# "register" (the Register tab at the same moment), "empty" (Live after
# Revoke all). SB_SPOTS are the points a story can click, in panel
# coordinates. With stag="x", the parts that rise in when the panel opens
# get ids x-0 (the header), x-1, x-2, ... in the order the page staggers
# them.

SB_W, SB_H = 460, 640
SB_PANEL, SB_TILE, SB_TILE2, SB_WELL = "#2e3034", "#37393d", "#353a41", "#1d2024"
SB_LINE, SB_LINE2 = "rgba(76,86,106,.55)", "#4c566a"
SB_INK, SB_FG, SB_DIM, SB_MUTE = "#e5e9f0", "#c5c8c6", "#838686", "#666d78"
SB_ACCENT, SB_ACCENT_INK, SB_SEL = "#88c0d0", "#1d2024", "#3b4045"
SB_GREEN, SB_AMBER, SB_RED, SB_BLUE, SB_QUOTE = "#a3be8c", "#ebcb8b", "#bf616a", "#81a1c1", "#d8c79c"
SB_DEFS = ('<filter id="sb-shadow" x="-30%" y="-20%" width="160%" height="150%">'
           '<feDropShadow dx="0" dy="24" stdDeviation="24" flood-color="#000" flood-opacity=".7"/></filter>')

# 15:20, Thursday. The deploy's grant has 12 minutes left; Jira came from the
# phone at 14:51; the homelab bundle was granted from the panel at 15:02.
SB_GRANTS = [
    dict(names="CLOUDFLARE_API_TOKEN", left="12m", frac=12 / 60, color=SB_AMBER,
         who=[("claude in ~/Sites/bubble-and-breakfast/web · ", None), ("Deploying bnb.twinspear.com", "q")],
         use="6 runs · last 47m ago", spark=[6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]),
    dict(names="JIRA_API_TOKEN", left="31m", frac=31 / 60, color=SB_ACCENT,
         who=[("claude in /tmp/dtcd-831 · ", None), ("DTCD-704 QA: Jira evidence", "q"), (" · via phone pixel", None)],
         use="23 runs · last 40s ago", spark=[0, 1, 2, 3, 1, 4, 2, 3, 5, 2, 4, 3]),
    dict(names="5 secrets", bundle="homelab", left="1h42", frac=102 / 120, color=SB_ACCENT,
         who=[("you, from Signal Box · ", None), ("From Signal Box: the homelab bundle", "q")],
         use="38 runs · last 2m ago", spark=[2, 5, 4, 6, 3, 4, 2, 5, 3, 2, 1, 1]),
]
SB_STRIP_TEXT = [("7 secrets in 3 grants · next to end: ", None), ("CLOUDFLARE_API_TOKEN", "b"), (" in ", None), ("12m", None)]
SB_NOTE = "A live secret works for any agent on this machine, not only the one that asked."

# The Register tab: newest first, the last 24 hours.
SB_EPISODES = [
    ("15:02", "grant", "Granted PROWLARR_API_KEY, RADARR_API_KEY, SABNZBD_API_KEY +2",
     [("for 2h · Signal Box in ~ · ", None), ("From Signal Box: the homelab bundle", "i")]),
    ("14:52", "run", "JIRA_API_TOKEN · acli",
     [("23 runs, all ok · claude in /tmp/dtcd-831 · 27m", None), (" · running", "l")]),
    ("14:51", "grant", "Granted JIRA_API_TOKEN",
     [("for 1h via phone pixel · claude in /tmp/dtcd-831 · ", None), ("DTCD-704 QA: Jira evidence", "i")]),
    ("14:32", "run", "CLOUDFLARE_API_TOKEN · npm",
     [("6 runs, all ok · claude in ~/Sites/bubble-and-breakfast/web · 1m", None)]),
    ("14:32", "grant", "Granted CLOUDFLARE_API_TOKEN",
     [("for 1h · claude in ~/Sites/bubble-and-breakfast/web · ", None), ("Deploying bnb.twinspear.com", "i")]),
]
SB_KIND = {"run": SB_ACCENT, "grant": SB_GREEN, "missed": SB_AMBER, "denied": SB_RED, "revoke": SB_BLUE, "lock": SB_BLUE}


def sb_day_marks():
    """The day strip's marks as (hours since 15:20 yesterday, kind): runs as
    ticks, the rest as dots. Made up, but shaped like a working day."""
    marks, seed = [], 7

    def rnd():
        nonlocal seed
        seed = (seed * 1103515245 + 12345) % 2 ** 31
        return seed / 2 ** 31

    def at(hh, mm):  # today's clock time, in hours since 15:20 yesterday
        return 24 - (15 + 20 / 60 - hh - mm / 60)

    for start, end, n in [(8.4, 9.6, 14), (10.1, 11.7, 22), (13.2, 13.9, 9), (14 + 32 / 60, 14 + 33 / 60, 6),
                          (14 + 52 / 60, 15 + 19 / 60, 23), (15 + 2 / 60, 15 + 18 / 60, 16), (20.5, 21.4, 7)]:
        for _ in range(n):
            h = start + rnd() * (end - start)
            marks.append((at(0, 0) + h if h < 15.34 else h - 15 - 20 / 60, "run"))
    marks += [(at(8, 20), "grant"), (at(10, 5), "grant"), (at(12, 31), "missed"), (at(12, 2), "revoke"),
              (at(14, 32), "grant"), (at(14, 51), "grant"), (at(15, 2), "grant"), (21.4 - 15 - 20 / 60, "grant")]
    return marks


SB_TABS = [("Live", 17, 53.2), ("Asks", 72.2, 53.2), ("Register", 127.4, 65.6), ("Vault", 195, 62.4)]
SB_SIBS = [("Yard ↗", 259.4, 69.2, SB_MUTE), ("Roundhouse ↗", 330.6, 112.4, SB_GREEN)]  # switchyard idle, roundhouse calm
SB_FOOT = {
    "live": [("r", "revoke"), ("R", "revoke all"), ("e", "+1h"), ("j/k", "select"), ("h", "bundle"), ("esc", "close")],
    "register": [("/", "filter"), ("tab", "next tab"), ("esc", "close")],
}
SB_STATES = {
    "live": dict(tab="Live", chip=("7 live", SB_ACCENT, "rgba(136,192,208,.14)"), counts=(3, 0, "", 38), foot="live"),
    "register": dict(tab="Register", chip=("7 live", SB_ACCENT, "rgba(136,192,208,.14)"), counts=(3, 0, "", 38), foot="register"),
    "empty": dict(tab="Live", chip=("nothing live", SB_GREEN, "rgba(163,190,140,.14)"), counts=(0, 0, "", 38), foot="live"),
}
SB_MAIN_TOP = 100.6
SB_SPOTS = {
    "tab:live": (43.6, 82.9), "tab:asks": (98.8, 82.9), "tab:register": (160.2, 82.9), "tab:vault": (226.2, 82.9),
    "yard": (294, 82.9), "roundhouse": (386.8, 82.9),
    "revoke-all": (384.5, 137.0),        # live
    "revoke:0": (403.3, 193.8), "extend:0": (403.3, 223.5),  # live, the first row
    "grant-bundle": (401.8, 283.8),      # empty
}


def sb_main_bottom(h):
    return h - 1 - 36.9


def sb_wrap(runs, chars, first=0):
    """Lays (text, style) runs into lines of at most `chars` characters as
    the panel's CSS wraps them: at spaces and after a hyphen before a
    letter, and anywhere in a word too long for a line. `first` is how many
    characters the first line has used already."""
    lines, line, used, gap = [], [], first, None
    for s, st in runs:
        for tok in re.findall(r" +|[^ ]+?-(?=[A-Za-z])|[^ ]+", s):
            if tok.isspace():
                if line:
                    gap = (" ", st)
                continue
            need = len(tok) + (1 if gap else 0)
            if used + need > chars and line:
                lines.append(line)
                line, used, gap = [], 0, None
                need = len(tok)
            while used + need > chars:  # longer than a line: break anywhere
                cut = chars - used
                line.append((tok[:cut], st))
                lines.append(line)
                line, used, tok, need = [], 0, tok[cut:], len(tok) - cut
            if gap:
                line.append(gap)
            line.append((tok, st))
            used += need
            gap = None
    if line:
        lines.append(line)
    merged = []
    for ln in lines:
        out = []
        for s, st in ln:
            if out and out[-1][1] == st:
                out[-1] = (out[-1][0] + s, st)
            else:
                out.append((s, st))
        merged.append(out)
    return merged


def sb_lines(x, y, lines, size, base, looks, lh, first_x=None):
    """Wrapped lines as <text>, one per line; looks maps a run's style to
    (fill, weight, italic)."""
    out = ""
    for i, ln in enumerate(lines):
        parts = [(s, *looks.get(st, (base, 400, False))) for s, st in ln]
        out += spans(first_x if (i == 0 and first_x is not None) else x, y + i * lh, parts, size)
    return out


def sb_button(x, y, w, label, kbd=None, kind=""):
    fill, line, color = SB_TILE2, SB_LINE2, SB_FG
    if kind == "go":
        fill, line, color = "rgba(163,190,140,.14)", "rgba(163,190,140,.45)", SB_GREEN
    out = rect(x, y, w, 24.7, 6, fill, line)
    if kbd:
        out += spans(x + 10, y + 16.5, [(label, color), (" ", color), (kbd, SB_MUTE)], 11.5)
    else:
        out += text(x + w / 2, y + 16.5, label, 11.5, color, anchor="middle")
    return out


def sb_ring(cx, cy, frac, label, color):
    c = 2 * 3.141592653589793 * 18
    return (f'<circle cx="{cx}" cy="{cy}" r="18" fill="none" stroke="{SB_WELL}" stroke-width="4"/>'
            f'<circle cx="{cx}" cy="{cy}" r="18" fill="none" stroke="{color}" stroke-width="4" stroke-linecap="round" '
            f'stroke-dasharray="{c:.2f}" stroke-dashoffset="{c * (1 - frac):.2f}" transform="rotate(-90 {cx} {cy})"/>'
            + text(cx, cy + 3.6, label, 10, SB_INK, anchor="middle"))


def sb_spark(x, y, vals, color, w=80, h=14):
    mx, n = max(1, *vals), len(vals)
    pts = [(x + i / (n - 1) * (w - 2) + 1, y + h - 1 - v / mx * (h - 3)) for i, v in enumerate(vals)]
    d = "M" + " L".join(f"{a:.1f} {b:.1f}" for a, b in pts)
    return (f'<path d="{d} L{x + w - 1} {y + h} L{x + 1} {y + h} Z" fill="{color}" fill-opacity=".14"/>'
            f'<path d="{d}" fill="none" stroke="{color}" stroke-width="1.5" stroke-linejoin="round"/>'
            f'<circle cx="{x + w - 1}" cy="{pts[-1][1]:.1f}" r="2" fill="{color}"/>')


def sb_stag(stag, i, body):
    """Wraps a part that rises in when the panel opens."""
    if not stag:
        return body
    return f'<g id="{stag}-{i}" style="transform-box:fill-box;transform-origin:center">{body}</g>'


def sb_frame(h=SB_H):
    return f'<g filter="url(#sb-shadow)">{rect(0, 0, SB_W, h, 14, SB_PANEL, SB_LINE)}</g>'


def sb_chrome(state, h=SB_H, stag=None):
    """The header, the tabs and the footer for a state."""
    st = SB_STATES[state]
    chip, chip_fg, chip_bg = st["chip"]
    cw = mono_w(chip, 11.5) + 20
    head = (f'<circle cx="22.5" cy="25.8" r="4.5" fill="{SB_GREEN}"/>'
            + text(36, 30.7, "SIGNAL BOX", 13.5, SB_INK, 700, spacing=1.62)
            + text(18, 51.1, "frostyarch · vault: hardware · YubiKey", 11.5, SB_DIM)
            + rect(442 - cw, 16, cw, 22.7, 11.35, chip_bg)
            + text(442 - cw / 2, 31.5, chip, 11.5, chip_fg, anchor="middle"))
    out = sb_stag(stag, 0, head)
    # The tabs: a well, the selected one filled with the accent.
    out += rect(14, 65.2, 432, 35.4, 9, SB_WELL)
    for (name, x, w), n in zip(SB_TABS, st["counts"]):
        on = name == st["tab"]
        lw = mono_w(name, 12) + (4 + mono_w(str(n), 12) if n != "" else 0)
        tx = x + (w - lw) / 2
        if on:
            out += rect(x, 68.2, w, 29.4, 6, SB_ACCENT)
            look = f'fill="{SB_ACCENT_INK}" font-weight="700"'
            num = f'<tspan dx="4" fill-opacity=".7">{n}</tspan>'
        else:
            look = f'fill="{SB_DIM}"'
            num = f'<tspan dx="4" fill="{SB_MUTE}">{n}</tspan>'
        out += f'<text x="{tx:.1f}" y="87.2" font-size="12" {look}>{name}{num if n != "" else ""}</text>'
    for label, x, w, dot in SB_SIBS:
        out += f'<circle cx="{x + 10.5:g}" cy="81.7" r="3.5" fill="{dot}"/>' + text(x + 19, 87.2, label, 12, SB_DIM)
    # The footer: the keys for this tab.
    top = h - 37.9
    out += f'<line x1="1" y1="{top + 0.5:g}" x2="{SB_W - 1}" y2="{top + 0.5:g}" stroke="{SB_LINE}"/>'
    x = 18
    for k, label in SB_FOOT[st["foot"]]:
        out += spans(x, h - 16, [(k, SB_FG, 500), (" " + label, SB_MUTE)], 11)
        x += mono_w(k + " " + label, 11) + 14
    return out


def sb_body(state, h=SB_H, stag=None, clip="sb-main"):
    """The tab's content, clipped to the panel's scrolling <main>."""
    out = {"live": sb_live, "register": sb_register, "empty": sb_empty}[state](stag)
    bottom = sb_main_bottom(h)
    return (f'<clipPath id="{clip}"><rect x="1" y="{SB_MAIN_TOP}" width="{SB_W - 2}" height="{bottom - SB_MAIN_TOP:.1f}"/></clipPath>'
            f'<g clip-path="url(#{clip})">{out}</g>')


def sb_panel(state="live", h=SB_H, stag=None, clip="sb-main"):
    """The whole panel for one state, at (0, 0)."""
    return sb_frame(h) + sb_chrome(state, h, stag) + sb_body(state, h, stag, clip)


def sb_bundles(y, live, stag, i):
    """The Bundles section: its heading and the homelab strip."""
    out = sb_stag(stag, i, text(16, y + 17.4, "BUNDLES", 10.5, SB_MUTE, spacing=1.26)
                  + text(444, y + 17.4, "ONE TOUCH EACH", 10.5, SB_MUTE, anchor="end", spacing=1.26))
    y += 29.2
    parts = [("homelab", SB_INK, 500), (" · 5 secrets for 2h", SB_FG)] + ([(" · live", SB_ACCENT)] if live else [])
    strip = (rect(14, y, 432, 40.7, 9, SB_WELL, SB_LINE) + spans(25, y + 24.6, parts, 12)
             + sb_button(368.6, y + 8, 66.4, "Renew" if live else "Grant", "h", "" if live else "go"))
    return out + sb_stag(stag, i + 1, strip)


def sb_live(stag):
    lines = sb_wrap(SB_STRIP_TEXT, 41)
    strip = (rect(14, 111.6, 432, 50.8, 9, SB_WELL, SB_LINE)
             + sb_lines(25, 132.6, lines, 12, SB_FG, {"b": (SB_INK, 500, False)}, 17.4)
             + sb_button(334.1, 124.7, 100.9, "Revoke all", "R"))
    out = sb_stag(stag, 1, strip)
    y = 170.4
    for i, g in enumerate(SB_GRANTS):
        who = sb_wrap(g["who"], 40)
        th = 11 + 49.8 + 3 + len(who) * 16.675 + 3 + 15.9 + 11
        c = y + 11  # the tile's content box
        if g.get("bundle"):
            name = (rect(81.5, c + 17, 55.1, 16.2, 4, "none", "rgba(136,192,208,.4)")
                    + text(87, c + 28.9, g["bundle"], 10.5, SB_ACCENT, 500) + text(143.1, c + 29.6, g["names"], 13, SB_INK, 700))
        else:
            name = text(81, c + 29.6, g["names"], 13, SB_INK, 700)
        use_y = c + 52.8 + len(who) * 16.675 + 3
        tile = (rect(14, y, 432, th, 10, SB_SEL if i == 0 else SB_TILE, SB_LINE2 if i == 0 else SB_LINE)
                + sb_ring(48, c + 22, g["frac"], g["left"], g["color"]) + name
                + sb_lines(81, c + 52.8 + 12.5, who, 11.5, SB_DIM, {"q": (SB_QUOTE, 400, True)}, 16.675)
                + sb_spark(81, use_y + 0.9, g["spark"], g["color"])
                + text(170, use_y + 11.9, g["use"], 11, SB_MUTE)
                + sb_button(372.6, c, 61.4, "Revoke") + sb_button(372.6, c + 29.7, 61.4, "+1h"))
        out += sb_stag(stag, 2 + i, tile)
        y += th + 8
    y += 11
    note = sb_wrap([(SB_NOTE, None)], 64)
    out += sb_stag(stag, 5, sb_lines(16, y + 11.9, note, 11, SB_MUTE, {}, 15.95))
    y += len(note) * 15.95 + 11 + 8
    return out + sb_bundles(y, True, stag, 6)


def sb_empty(stag):
    lines = sb_wrap([("Agents that need a private secret will ask, and the lamp will tell you.", None)], 51)
    tile = (rect(14, 111.6, 432, 114.5, 10, SB_TILE, SB_LINE)
            + text(230, 152.7, "Nothing is granted.", 13, SB_INK, 500, "middle"))
    for i, ln in enumerate(lines):
        tile += text(230, 175.6 + i * 18.85, ln[0][0], 13, SB_DIM, anchor="middle")
    return sb_stag(stag, 1, tile) + sb_bundles(234.2, False, stag, 2)


def sb_register(stag):
    # The last 24 hours: runs as ticks, grants, misses and revokes as dots.
    g = [f'<g transform="translate(24,119.6) scale({414 / 420:.5f})">']
    xs = lambda hrs: 10 + hrs / 24 * 400
    color = {"grant": SB_GREEN, "missed": SB_AMBER, "revoke": SB_BLUE, "denied": SB_RED}
    for hrs, kind in sb_day_marks():
        if kind == "run":
            g.append(f'<line x1="{xs(hrs):.1f}" x2="{xs(hrs):.1f}" y1="18" y2="33" stroke="{SB_ACCENT}" stroke-opacity=".5"/>')
    for hrs, kind in sb_day_marks():
        if kind != "run":
            g.append(f'<circle cx="{xs(hrs):.1f}" cy="43" r="3.2" fill="{color[kind]}"/>')
    for k, label in enumerate(["18", "00", "06", "12"]):
        g.append(text(xs(2 + 40 / 60 + 6 * k), 56, label, 9, SB_MUTE, anchor="middle"))
    g.append(f'<line x1="{xs(24)}" x2="{xs(24)}" y1="10" y2="47" stroke="{SB_INK}" stroke-opacity=".45" stroke-dasharray="2 2"/>')
    g.append(text(xs(24) - 2, 8, "now", 9, SB_FG, anchor="end") + "</g>")
    out = sb_stag(stag, 1, rect(14, 111.6, 432, 72.9, 10, SB_TILE, SB_LINE) + "".join(g))
    out += sb_stag(stag, 2, rect(14, 192.5, 432, 32.8, 8, SB_WELL, SB_LINE) + text(24, 213.6, "/", 13, SB_MUTE)
                   + text(41.8, 213.6, "filter by secret, directory or agent", 13, "#7b7f86"))
    y = 233.4
    for when, kind, head, sub in SB_EPISODES:
        hl = sb_wrap([(head, None)], 51, first=2)
        sl = sb_wrap(sub, 51)
        th = 18 + (len(hl) + len(sl)) * 16.675 + 1
        out += (rect(14, y, 432, th, 10, SB_TILE, SB_LINE) + text(25, y + 21.5, when, 11.5, SB_MUTE)
                + f'<circle cx="84.5" cy="{y + 16.5:g}" r="3.5" fill="{SB_KIND[kind]}"/>'
                + sb_lines(81, y + 21.5, hl, 11.5, SB_INK, {}, 16.675, first_x=95)
                + sb_lines(81, y + 21.5 + len(hl) * 16.675 + 1, sl, 11.5, SB_DIM,
                           {"i": (SB_DIM, 400, True), "l": (SB_ACCENT, 400, False), "a": (SB_AMBER, 400, False)}, 16.675))
        y += th + 8
    return out


def sb_toast(cx, bottom, msg):
    """The panel's toast, centered on cx with its bottom edge at bottom (the
    page puts it 24px off the bottom of the board's surface)."""
    w = mono_w(msg, 12) + 26
    return rect(cx - w / 2, bottom - 33, w, 33, 8, SB_WELL, SB_LINE2) + text(cx, bottom - 12.2, msg, 12, SB_INK, anchor="middle")


# ---------------------------------------------------------------- the request window

def request_window(done=False):
    """broker-dialog's request window for CLOUDFLARE_API_TOKEN, 470 wide, its
    top-left at (0, 0): waiting for a touch (505 tall) or granted (435 tall,
    the For row plain and the footer gone)."""
    h = RH_DONE if done else RH
    g = [f'<g font-family="{SANS}">',
         f'<g filter="url(#shadow)">{rect(0, 0, RW, h, 12, DLG_BG, "#2a2c33")}</g>',
         rect(22, 20.9, 38, 38, 10, DLG_PANEL),
         f'<g transform="translate(31,29.9) scale(.8333)" fill="none" stroke="{DLG_PRIMARY}" stroke-width="2" '
         'stroke-linecap="round" stroke-linejoin="round"><path d="M15.5 7.5 19 4"/><path d="m21 2-9.6 9.6"/>'
         '<path d="M17 6l3 3"/><circle cx="7.5" cy="15.5" r="5.5"/></g>',
         text(72, 36.3, "An agent wants to use a secret", 15, DLG_FG, 600),
         text(72, 56.1, "broker request" if done else "broker request · expires in 1:58", 12, DLG_MUTED),
         text(22, 87.8, "SECRET", 11, DLG_FAINT, spacing=0.66),
         rect(22, 97.2, RW - 44, 32.8, 8, DLG_PANEL),
         text(32, 118.5, "CLOUDFLARE_API_TOKEN", 12, DLG_FG, family=FONT),
         text(RW - 32, 119.3, "Cloudflare API, DNS and Workers", 12, DLG_MUTED, anchor="end"),
         text(22, 158, "REQUEST", 11, DLG_FAINT, spacing=0.66),
         text(22, 181.5, "For", 13, DLG_FAINT)]
    if done:
        g.append(text(124, 181.5, "1 hour, until 15:32", 13, DLG_FG))
    else:
        g += [rect(124, 167.4, 124, 26.2, 6, DLG_PANEL, DLG_LINE), text(133, 185.5, "1 hour (asked)", 13, DLG_FG),
              f'<path d="M231 181.5l4 4 4-4" fill="none" stroke="{DLG_MUTED}" stroke-width="1.5"/>',
              text(251.5, 185.6, "until 15:32", 12, DLG_MUTED)]
    y = 191.6 if done else 199.6  # the For row is 8px shorter without the select
    for i, (k, v) in enumerate([("Asked by", "bash < zsh < claude < t3code"), ("Directory", "~/Sites/bubble-and-breakfast/web"),
                                ("Will run", "npm run deploy")]):
        yy = y + i * 24.15
        g += [text(22, yy + 14.1, k, 13, DLG_FAINT), text(124, yy + 13.4, v, 12, DLG_FG, family=FONT)]
    c = 272.1 if done else 280.1
    g += [rect(22, c, RW - 44, 51.6, 6, CLAIM_LINE), rect(25, c, RW - 47, 51.6, 6, CLAIM_BG), rect(25, c, 8, 51.6, 0, CLAIM_BG),
          text(35, c + 20, "The agent says (unverified)", 11, CLAIM_FG),
          text(35, c + 37.5, "Deploying bnb.twinspear.com", 13, DLG_FG),
          text(22, c + 76.7, "While granted, any agent on this machine can use it, not just this one.", 12, DLG_MUTED)]
    if done:
        g += [rect(22, 368.5, RW - 44, 48, 10, DLG_PANEL, DLG_LINE),
              f'<circle cx="48" cy="392.5" r="11" fill="{OK}"/>',
              '<path d="M43 392.8 l3.4 3.6 6.4-7.4" fill="none" stroke="#fff" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>',
              text(69, 397.9, "Granted for 1h", 14, DLG_FG, 600)]
    else:
        g += [rect(22, 376.5, RW - 44, 62.4, 10, TOUCH_BG, TOUCH_LINE),
              f'<circle cx="43" cy="407.7" r="12" fill="none" stroke="{OK}" stroke-width="2" class="pulse" '
              'style="transform-box:fill-box;transform-origin:center"/>',
              f'<circle cx="43" cy="407.7" r="6" fill="{OK}"/>',
              text(61, 404.7, "Touch your YubiKey to approve", 14, DLG_FG, 600),
              text(61, 422.2, "or press Deny to turn it down", 12, DLG_MUTED),
              rect(386.5, 454.9, 61.5, 32.2, 8, DLG_BUTTON), text(417.25, 476, "Deny", 13, DLG_FG, anchor="middle")]
    return "".join(g) + "</g>"


# ---------------------------------------------------------------- the timeline

# tracks[id] is a list of (seconds, {css property: value}, timing function).
tracks = defaultdict(list)


def key(el, t, ease=None, **props):
    """At absolute time t, el has props (underscores become hyphens). ease is
    the timing function for the stretch that starts at this key."""
    tracks[el].append((t, {k.replace("_", "-"): v for k, v in props.items()}, ease))


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


def cut(old, new, t):
    """Replaces old with new at t, the way waybar and the panel redraw."""
    swap(old, new, t - 0.01, fade=0.01)


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


ripples = []


def ripple(t, x, y, color=ACCENT):
    """A click: a ring that grows and fades where the cursor pressed."""
    rid = f"ripple{len(ripples)}"
    ripples.append(rid)
    add("overlay", f'<circle id="{rid}" cx="{x:.1f}" cy="{y:.1f}" r="18" fill="none" stroke="{color}" stroke-width="2.5" '
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
        key("cursor", t - travel, ease="cubic-bezier(.45,0,.25,1)", transform=f"translate({self.x:.1f}px,{self.y:.1f}px)")
        key("cursor", t, transform=f"translate({x:.1f}px,{y:.1f}px)")
        self.x, self.y = x, y

    def click(self, t, x, y, travel=0.55):
        self.go(t - 0.05, x, y, travel)
        ripple(t, x, y)


cursor = Cursor(*CURSOR_HOME)


def typed(layer, id, x, y, s, t0, t1, size=13, parts=None, clip="term-clip"):
    """A line typed out between t0 and t1: a cover the color of the window
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
    # Neovide behind, so the windows have something to sit over.
    x, y, w, h = 560, 66, 600, 300
    add("desk", rect(x, y, w, h, 12, "#191c21", "#2a2e35"))
    add("desk", rect(x, y, w, 30, 12, TERM_TOP))
    for k, c in enumerate(["#bf616a", "#ebcb8b", "#a3be8c"]):
        add("desk", f'<circle cx="{x + 18 + k * 16}" cy="{y + 15}" r="5" fill="{c}" fill-opacity=".45"/>')
    bars = [(0, .42), (24, .55), (48, .38), (24, .62), (0, .3), (24, .5), (48, .7), (48, .44), (24, .36), (0, .2)]
    cols = ["#3b4252", "#434c5e", "#5e81ac55", "#88c0d040", "#4c566a"]
    for i, (indent, f) in enumerate(bars):
        add("desk", rect(x + 22 + indent, y + 50 + i * 24, int(w * f), 9, 4, cols[i % len(cols)]))


def line_y(i):
    return TY + 58 + i * 22


def build_terminal():
    """T3 Code, with claude working in its thread."""
    add("windows", f'<g filter="url(#shadow)">{rect(TX, TY, TW, TH, 12, TERM_BG, "#2b2f36")}</g>')
    add("windows", rect(TX, TY, TW, 30, 12, TERM_TOP))
    add("windows", rect(TX, TY + 18, TW, 12, 0, TERM_TOP))
    for k, c in enumerate(["#bf616a", "#ebcb8b", "#a3be8c"]):
        add("windows", f'<circle cx="{TX + 18 + k * 16}" cy="{TY + 15}" r="5" fill="{c}" fill-opacity=".6"/>')
    add("windows", text(TX + TW / 2, TY + 20, "T3 Code · bubble-and-breakfast", 11.5, DIM, anchor="middle"))
    x = TX + 22
    add("windows", spans(x, line_y(0), [("~/Sites/bubble-and-breakfast/web", "#6b7380"), ("  main", ACCENT)]))
    add("windows", spans(x, line_y(1), [("❯ ", CALM), ("ship the night look", FG)]))


def build_request():
    """The request window, waiting and granted, opening from its middle."""
    add("request", f'<g id="request" style="transform-origin:{RX + RW / 2}px {RY + RH / 2}px">'
                   f'<g transform="translate({RX},{RY})">'
                   f'<g id="req-wait">{request_window(False)}</g>'
                   f'<g id="req-done" opacity="0">{request_window(True)}</g></g></g>')


def build_yubikey():
    x, y = 872, 380
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


# The bar's geometry, filled in by build_bar(): where the broker module sits.
LAMP = {}
COUNT_W = 2 * CW  # " 7" after the pie
SLICES = {s: f"bl-s{s}" for s in (8, 6, 4, 2)}  # the pie glyphs the story shows


def pill(x, w, id=None, style=None):
    return rect(x, BAR_Y + 0.5, w, BAR_H - 1, 10, PILL, PILL_LINE, id=id, style=style)


def build_bar():
    """The Platform waybar from dot-files' config.jsonc: workspaces and the
    window title on the left; media, lamps, audio, desk, clock and tray on
    the right, each group a pill."""
    b = []
    # Workspaces: a dim number, then a glyph per window.
    x = 6
    spaces = [("1", ["t3", "neovide"], True), ("2", ["chrome", "slack"], False)]
    ws_w = 2 + 8 + sum(16 + CW + len(g) * (CW + ICON) + 2 for _, g, _ in spaces)
    b.append(pill(x, ws_w))
    bx = x + 5
    for num, gl, on in spaces:
        w = 16 + CW + len(gl) * (CW + ICON)
        bx += 1
        if on:
            b.append(rect(bx, BAR_Y + 4, w, BAR_H - 8, 7, WS_ON))
        col = B_BRIGHT if on else B_FG
        b.append(text(bx + 8, BAR_BASE, num, BAR_SIZE, col, style="opacity:.55"))
        gx = bx + 8 + CW
        for name in gl:
            b.append(glyph(name, gx + CW + ICON / 2, BAR_MID, col))
            gx += CW + ICON
        bx += w + 1
    # The window title: T3 Code's glyph from the rewrite, or the request window's own title.
    wx = x + ws_w + 6
    t3_w = 2 + 24 + ICON + 9 * CW
    b.append(f'<g id="win-t3">{pill(wx, t3_w)}{glyph("t3", wx + 13 + ICON / 2, BAR_MID, B_BRIGHT)}'
             f'{text(wx + 13 + ICON + 2 * CW, BAR_BASE, "T3 Code", BAR_SIZE, B_BRIGHT)}</g>')
    b.append(f'<g id="win-req" opacity="0">{pill(wx, 2 + 24 + 14 * CW)}'
             f'{text(wx + 13, BAR_BASE, "broker request", BAR_SIZE, B_BRIGHT)}</g>')

    # Right side, packed from the edge.
    r = W - 6
    tray_w = 2 + 20 + 3 * 16 + 2 * 8
    r -= tray_w
    b.append(pill(r, tray_w))
    for k, (name, col) in enumerate([("slack", "#e01e5a"), ("discord", "#7289da"), ("bluetooth", "#5e81ac")]):
        b.append(glyph(name, r + 11 + 8 + k * 24, BAR_MID, col, 16))
    clock_w = 2 + 24 + 17 * CW
    r -= 6 + clock_w
    b.append(pill(r, clock_w))
    for i, hhmm in enumerate(["14:32", "14:51", "15:02", "15:20"]):
        b.append(text(r + 13, BAR_BASE, f"Thu Nov 12  {hhmm}", BAR_SIZE, B_PURPLE, id=f"clock{i}",
                      style=None if i == 0 else "opacity:0"))
    desk_w = 2 + 8 + 2 * (16 + ICON)
    r -= 6 + desk_w
    b.append(pill(r, desk_w))
    b.append(glyph("mic", r + 5 + 8 + ICON / 2, BAR_MID, B_DIM))           # Logbook, nothing listening
    b.append(glyph("chevron", r + 5 + 30 + 8 + ICON / 2, BAR_MID, B_DIM))  # the drawer's handle
    aud = 16 + ICON + 4 * CW
    audio_w = 2 + 8 + 2 * aud
    r -= 6 + audio_w
    b.append(pill(r, audio_w))
    b.append(glyph("volume", r + 5 + 8 + ICON / 2, BAR_MID, B_FROST) + text(r + 5 + 8 + ICON + CW, BAR_BASE, "42%", BAR_SIZE, B_FROST))
    b.append(glyph("mouse", r + 5 + aud + 8 + ICON / 2, BAR_MID, B_DIM) + text(r + 5 + aud + 8 + ICON + CW, BAR_BASE, "80%", BAR_SIZE, B_DIM))

    # The lamps: Roundhouse, Switchyard, Signal Box, VPN, do not disturb.
    lamps_r = r - 6
    mod = 16 + ICON
    narrow = 2 + 8 + 5 * mod
    b.append(pill(lamps_r - narrow, narrow, id="lamps-narrow"))
    b.append(pill(lamps_r - narrow - COUNT_W, narrow + COUNT_W, id="lamps-wide", style="opacity:0"))
    cx = lamps_r - 5 - mod / 2
    b.append(glyph("bell", cx, BAR_MID, B_DIM))
    b.append(glyph("vpn_off", cx - mod, BAR_MID, B_DIM))
    broker_r = lamps_r - 5 - 2 * mod  # the broker module's right edge
    g0 = broker_r - 8 - ICON / 2       # its glyph with no count beside it
    LAMP.update(x=g0, right=broker_r, count_x=broker_r - 8 - CW)
    # Hover: the module's tint while the cursor is on it, without and with a count.
    b.append(rect(broker_r - mod, BAR_Y + 3, mod, BAR_H - 6, 7, HOVER, id="bh-narrow", style="opacity:0"))
    b.append(rect(broker_r - mod - COUNT_W, BAR_Y + 3, mod + COUNT_W, BAR_H - 6, 7, HOVER, id="bh-wide", style="opacity:0"))
    for n in "127":
        b.append(text(LAMP["count_x"], BAR_BASE, n, BAR_SIZE, B_FROST, id=f"bc-{n}", style="opacity:0"))
    # Everything left of the count moves over while there is one.
    b.append('<g id="lamps-shift">')
    b.append(glyph("broker_quiet", g0, BAR_MID, B_DIM, id="bl-quiet"))
    b.append(f'<g id="bl-key" opacity="0">{glyph("broker_key", g0, BAR_MID, B_YELLOW, cls="ask")}</g>')  # pulses while asking
    for s, el in SLICES.items():
        b.append(glyph(f"slice{s}", g0, BAR_MID, B_FROST, id=el, style="opacity:0"))
    b.append(glyph("switchyard", g0 - mod, BAR_MID, B_FROST))
    b.append(glyph("roundhouse", g0 - 2 * mod, BAR_MID, B_GREEN))
    # Tannoy: the cover and what is playing, one pill.
    song = "Lofi Rails · Night Shift"
    media_w = 1 + 10 + 18 + 8 + len(song) * CW + 12 + 1
    mx = lamps_r - narrow - 6 - media_w
    b.append(pill(mx, media_w))
    b.append(f'<g transform="translate({mx + 11:.1f},{BAR_Y + 8})">'
             '<rect width="18" height="18" rx="3" fill="url(#cover)"/>'
             '<circle cx="12.5" cy="6" r="2.6" fill="#ebcb8b" fill-opacity=".9"/>'
             '<path d="M0 14 L5 10 L9 13 L13 9 L18 13 V15 a3 3 0 0 1 -3 3 H3 a3 3 0 0 1 -3 -3 Z" fill="#2e3440" fill-opacity=".85"/></g>')
    b.append(text(mx + 37, BAR_BASE, song, BAR_SIZE, B_YELLOW))
    b.append("</g>")
    LAMP["media_x"], LAMP["window_end"] = mx, wx + t3_w
    add("bar", "".join(b))


def lamp_x(count):
    """Where the broker glyph sits, with or without a count beside it."""
    return LAMP["x"] - (COUNT_W if count else 0)


def build_tooltip():
    """waybar's tooltip for the lamp, as `broker bar` writes it."""
    lines = ["Signal Box", "7 secrets live in 3 grants · CLOUDFLARE_API_TOKEN ends in 12m"]
    w = mono_w(lines[1], 12.5) + 28
    x = min(lamp_x(True) - w / 2, W - 8 - w)
    y = BAR_Y - 8 - 56
    add("overlay", f'<g id="tip" opacity="0">{rect(x, y, w, 56, 10, "rgba(40,42,46,.95)", PILL_LINE)}'
                   + text(x + 14, y + 23, lines[0], 12.5, B_FG) + text(x + 14, y + 42, lines[1], 12.5, B_FG) + "</g>")


# The panel on screen: placed against the bar, centered on the click and
# clamped 10px from the edges, as the page's place() does.
PX = 0  # set by build_panel()
PY = BAR_Y - 10 - SB_H


def build_panel():
    global PX
    ox = lamp_x(True)
    PX = min(max(ox - SB_W / 2, 10), W - SB_W - 10)
    # The board's surface stops at the bar, so the panel's shadow does too.
    p = [f'<g clip-path="url(#board-surface)"><g id="panel" opacity="0"><g id="panel-s" style="transform-origin:{ox:.1f}px {BAR_Y}px">',
         f'<g transform="translate({PX:.1f},{PY})">', sb_frame()]
    for state, base in [("live", "1"), ("register", "0"), ("empty", "0")]:
        p.append(f'<g id="pc-{state}" opacity="{base}">{sb_chrome(state, stag="sbh" if state == "live" else None)}</g>')
    for state, base in [("live", "1"), ("register", "0"), ("empty", "0")]:
        p.append(f'<g id="pb-{state}" opacity="{base}">{sb_body(state, stag="sbl" if state == "live" else None, clip="clip-" + state)}</g>')
    # Revoke all under the cursor: the page's .btn.warn:hover.
    p.append(rect(334.1, 124.7, 100.9, 24.7, 6, "none", SB_RED, id="ra-hover", style="opacity:0"))
    p.append("</g></g></g>")
    p.append(f'<g id="toast" opacity="0">{sb_toast(W / 2, BAR_Y - 24, "Revoked CLOUDFLARE_API_TOKEN and 6 more")}</g></g>')
    add("panel", "".join(p))


def build_cursor():
    add("cursor", '<g id="cursor">'
                  '<path d="M0,0 L0,22 L6,17 L10,26 L14,24 L10,15 L18,15 Z" fill="#eceff4" stroke="#1d2024" '
                  'stroke-width="1.4" stroke-linejoin="round"/></g>')


# ---------------------------------------------------------------- scenes

TERM_X = TX + 22
CMD1 = 'broker exec --request --reason "Deploying bnb.twinspear.com" \\'
CMD2 = "    CLOUDFLARE_API_TOKEN -- npm run deploy"
TERM_LINES = []  # ids of thread lines that a scene shows, so reset can hide them


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
    """The request window springs up and takes focus; the lamp turns to an
    amber key while the agent asks; the YubiKey blinks for a touch."""
    spring_open("request", t0 + 0.1, dur=0.6, dy=10, from_scale=0.82)
    cut("win-t3", "win-req", t0 + 0.15)
    cut("bl-quiet", "bl-key", t0 + 0.15)
    key("yubikey", t0 + 0.5, ease="cubic-bezier(.2,.8,.2,1)", opacity="0", transform="translateY(18px)")
    key("yubikey", t0 + 0.9, opacity="1", transform="translateY(0px)")


def count_on(t):
    """The lamp gains a count: the pill widens and its left part moves over."""
    cut("lamps-narrow", "lamps-wide", t)
    key("lamps-shift", t - 0.01, transform="translateX(0px)")
    key("lamps-shift", t, transform=f"translateX({-COUNT_W}px)")


def count_off(t):
    cut("lamps-wide", "lamps-narrow", t)
    key("lamps-shift", t - 0.01, transform=f"translateX({-COUNT_W}px)")
    key("lamps-shift", t, transform="translateX(0px)")


def scene_touch(t0, d):
    """A touch: the key flashes, the window says granted, the lamp fills."""
    key("yk-flash", t0 + 0.15, opacity="0", transform="scale(.8)")
    key("yk-flash", t0 + 0.2, opacity="1", transform="scale(.9)")
    key("yk-flash", t0 + 0.75, opacity="0", transform="scale(2.4)")
    cut("req-wait", "req-done", t0 + 0.3)
    swap("tl5", "tl5b", t0 + 0.45)
    add("windows", spans(TERM_X, line_y(5), [("  └ ", MUTE), ("granted for 1h · touched on the YubiKey", CALM)],
                         13, id="tl5b", style="opacity:0"))
    TERM_LINES.append("tl5b")
    cut("bl-key", SLICES[8], t0 + 0.45)
    key("bc-1", t0 + 0.44, opacity="0")
    key("bc-1", t0 + 0.45, opacity="1")
    count_on(t0 + 0.45)


def scene_deploy(t0, d):
    """The window closes a second after it says granted; focus goes back to
    T3 Code and the deploy runs, the token redacted."""
    spring_close("request", t0 + 0.2, dur=0.3, dy=0, to_scale=0.9)
    cut("win-req", "win-t3", t0 + 0.45)
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
    """Time passes. Jira's token arrives from the phone, the homelab bundle
    from the panel; the count climbs and the pie drains with the deploy's
    grant, the one that ends first."""
    for i, (a, s_old, s_new, n_old, n_new) in enumerate([(0.25, 8, 6, "1", "2"), (0.95, 6, 4, "2", "7"), (1.65, 4, 2, None, None)]):
        t = t0 + a
        swap(f"clock{i}", f"clock{i + 1}", t, fade=0.2)
        cut(SLICES[s_old], SLICES[s_new], t + 0.1)
        if n_old:
            cut(f"bc-{n_old}", f"bc-{n_new}", t + 0.1)


def tab_click(t, old, new, from_right):
    """A tab switch: the header and tabs redraw in place, the new body
    slides in a few pixels from the side it came from (no blanking)."""
    cut(f"pc-{old}", f"pc-{new}", t)
    key(f"pb-{old}", t - 0.01, opacity="1")
    key(f"pb-{old}", t, opacity="0")
    dx = 10 if from_right else -10
    key(f"pb-{new}", t - 0.01, opacity="0", transform=f"translateX({dx}px)")
    key(f"pb-{new}", t, ease="cubic-bezier(.2,.8,.3,1)", opacity=".6", transform=f"translateX({dx}px)")
    key(f"pb-{new}", t + 0.16, opacity="1", transform="translateX(0px)")


def scene_open(t0, d):
    """The cursor rests on the lamp, its tooltip says what is live, and a
    click springs the panel out of it on Live."""
    x = lamp_x(True)
    cursor.go(t0 + 0.5, x, BAR_MID, travel=0.55)
    show("bh-wide", t0 + 0.45, t0 + 2.1, fade=0.08)
    show("tip", t0 + 0.95, t0 + 1.55, fade=0.15)
    c = t0 + 1.6
    ripple(c, x, BAR_MID)
    o = c + 0.05
    key("panel-s", o, ease="cubic-bezier(.17,1.22,.32,1)", transform="scale(.06)")
    key("panel-s", o + 0.52, transform="scale(1)")
    key("panel", o, opacity="0")
    key("panel", o + 0.18, opacity="1")
    stag = ["sbh-0"] + [f"sbl-{i}" for i in range(1, 8)]
    for i, el in enumerate(stag):
        s = o + 0.11 + i * 0.038
        key(el, s, ease="cubic-bezier(.2,.9,.3,1)", opacity="0", transform="translateY(9px) scale(.985)")
        key(el, s + 0.4, opacity="1", transform="translateY(0px) scale(1)")
    cursor.go(t0 + d - 0.05, PX + 230, PY + 380, travel=0.5)


def scene_register(t0, d):
    """A look at Register: the last 24 hours, the deploy's grant and runs
    among them."""
    x, y = SB_SPOTS["tab:register"]
    t = t0 + 0.5
    cursor.click(t, PX + x, PY + y, travel=0.45)
    tab_click(t + 0.03, "live", "register", True)
    cursor.go(t0 + d - 0.1, PX + 250, PY + 300, travel=0.6)


def scene_revoke(t0, d):
    """Back to Live, then Revoke all: every grant drops, the lamp goes dim."""
    x, y = SB_SPOTS["tab:live"]
    t = t0 + 0.4
    cursor.click(t, PX + x, PY + y, travel=0.4)
    tab_click(t + 0.03, "register", "live", False)
    x, y = SB_SPOTS["revoke-all"]
    cursor.go(t0 + 1.15, PX + x, PY + y, travel=0.55)
    show("ra-hover", t0 + 1.12, fade=0.08)
    r = t0 + 1.45
    ripple(r, PX + x, PY + y, ACT)
    s = r + 0.25  # broker revokes and sends a new snapshot
    key("ra-hover", s - 0.01, opacity="1")
    key("ra-hover", s, opacity="0")
    cut("pc-live", "pc-empty", s)
    cut("pb-live", "pb-empty", s)
    cut(SLICES[2], "bl-quiet", s)
    key("bc-7", s - 0.01, opacity="1")
    key("bc-7", s, opacity="0")
    count_off(s)
    key("toast", s, ease="cubic-bezier(.2,.8,.2,1)", opacity="0", transform="translateY(10px)")
    key("toast", s + 0.22, opacity="1", transform="translateY(0px)")


def scene_close(t0, d):
    """A click on the lamp folds the panel back into it."""
    x = lamp_x(False)
    cursor.go(t0 + 0.45, x, BAR_MID, travel=0.5)
    show("bh-narrow", t0 + 0.4, t0 + 1.0, fade=0.08)
    c = t0 + 0.5
    ripple(c, x, BAR_MID)
    key("panel-s", c + 0.04, ease="cubic-bezier(.5,0,.75,0)", transform="scale(1)")
    key("panel-s", c + 0.28, transform="scale(.06)")
    key("panel", c + 0.08, opacity="1")
    key("panel", c + 0.28, opacity="0")
    key("toast", c + 0.04, opacity="1", transform="translateY(0px)")
    key("toast", c + 0.26, opacity="0", transform="translateY(10px)")


def scene_reset(t0, d):
    """Back to the first frame: the thread cleared, the clock back, the
    panel reset."""
    for lid in TERM_LINES:
        key(lid, t0, opacity="1" if lid != "tl5" else "0")
        key(lid, t0 + 0.3, opacity="0")
    for i in range(2):
        key(f"cmd{i}-cover", t0 + 0.35, transform=f"translateX({mono_w([CMD1, CMD2][i], 13) + 10:.0f}px)")
        key(f"cmd{i}-cover", t0 + 0.36, transform="translateX(0px)")
    swap("clock3", "clock0", t0 + 0.05, fade=0.3)
    # Everything else that ended somewhere other than where it started
    # (the panel's insides, the request window, ripples) is hidden by now,
    # so it snaps back to its first frame unseen.
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
    ("more-grants", 2.2, scene_more_grants),
    ("open", 2.8, scene_open),
    ("register", 2.4, scene_register),
    ("revoke", 2.6, scene_revoke),
    ("close", 1.3, scene_close),
    ("reset", 0.8, scene_reset),
]

STARTS, T = {}, 0.0
for _name, _sec, _fn in SCENES:
    STARTS[_name] = T
    T += _sec


def start(name):
    return STARTS[name]


# The reduced-motion still: the panel open on Live, every row in place.
STILL = start("open") + 2.7

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
    css.append(".pulse,.blink,.ask{animation:none!important}")
    return "\n".join(css)


BASE_CSS = """
text{white-space:pre}
.pulse{animation:pulse 1.4s ease-out infinite}
@keyframes pulse{0%{opacity:.7;transform:scale(.6)}80%,100%{opacity:0;transform:scale(1.7)}}
.blink{animation:blink 1.1s ease-in-out infinite}
@keyframes blink{50%{opacity:.35}}
.ask{animation:ask 1.2s ease-in-out infinite alternate}
@keyframes ask{to{fill:ASK_LOW}}
""".replace("ASK_LOW", B_ASK_LOW)

# The README's alt text says the same.
ALT = ("A Hyprland desktop under a floating waybar. An agent in T3 Code runs broker exec --request for "
       "CLOUDFLARE_API_TOKEN, broker's request window asks for a YubiKey touch while the Signal Box lamp in the "
       "bar turns to an amber key, and the deploy runs with the token redacted from its output. The lamp fills, "
       "and its count climbs to 7 as other grants arrive. A click on it springs the Signal Box panel out: three "
       "live grants with their time left, a look at the Register tab with the day's grants and runs, then Revoke "
       "all, and the lamp goes dim.")


def render(still=None):
    build_desk()
    build_terminal()
    build_request()
    build_yubikey()
    build_bar()
    build_tooltip()
    build_panel()
    build_cursor()
    for name, sec, fn in SCENES:
        fn(STARTS[name], sec)

    css = BASE_CSS + compile_css()
    if still is not None:
        css += "\n" + still_css(still)
    css += ("\n@media (prefers-reduced-motion: reduce){\n" + still_css(STILL)
            + "\n#cursor,[id^=ripple],#tip{display:none}\n}")

    head = (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" '
            f'font-family="{FONT}" role="img" aria-label="{esc(ALT)}">'
            "<title>credential-broker and Signal Box</title>"
            f"<desc>{esc(ALT)}</desc>"
            f"<style>{css}</style>"
            f"""<defs>
<linearGradient id="wall" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="{BG0}"/><stop offset="1" stop-color="{BG1}"/></linearGradient>
<radialGradient id="haze1" cx=".85" cy="0" r=".75"><stop offset="0" stop-color="{ACCENT}" stop-opacity=".13"/><stop offset="1" stop-color="{ACCENT}" stop-opacity="0"/></radialGradient>
<radialGradient id="haze2" cx=".05" cy="1" r=".6"><stop offset="0" stop-color="#b48ead" stop-opacity=".08"/><stop offset="1" stop-color="#b48ead" stop-opacity="0"/></radialGradient>
<linearGradient id="cover" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#5e81ac"/><stop offset="1" stop-color="#b48ead"/></linearGradient>
<filter id="shadow" x="-20%" y="-20%" width="140%" height="150%"><feDropShadow dx="0" dy="18" stdDeviation="22" flood-color="#000" flood-opacity=".6"/></filter>
{SB_DEFS}
<clipPath id="board-surface"><rect width="{W}" height="{BAR_Y}"/></clipPath>
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
    scenes = ", ".join(f"{n} {start(n):g}s" for n, s, _ in SCENES)
    print(f"wrote {a.out} ({len(svg.encode()) // 1024} KB, {T:g}s loop; scenes start at: {scenes})")


if __name__ == "__main__":
    main()

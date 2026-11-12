# The README hero

`artwork/hero.svg` is generated. Edit `hero.py` and run it again:

```sh
python3 artwork/hero/hero.py
```

It needs Python 3 and nothing else.

## How the loop is built

The SVG is one loop in two acts, on a desktop under the floating Platform waybar from dot-files. First broker: an agent in T3 Code asks for a secret, the request window springs up while the Signal Box lamp turns to an amber key, a YubiKey touch grants it, and the deploy runs with the token redacted. Then Signal Box: the lamp's pie drains while its count climbs, the panel opens out of the lamp on Live, a look at the Register tab, back to Live for Revoke all, and the panel folds back.

- **Objects** are drawn once by the `build_*` functions: the desk, the T3 Code window, the request window, the YubiKey, the bar and its lamp, the tooltip, the panel, the cursor. Every part that moves has an `id`. Nothing in them is animated by itself.
- **The two windows broker draws** have their own functions, which know nothing about the desk or the bar. `sb_panel(state, h)` draws Signal Box at its real size (460 wide, `h` tall, 640 at most) in its own coordinates, for the state `"live"`, `"register"` or `"empty"`; `sb_frame`, `sb_chrome` and `sb_body` are its parts, so a tab switch can redraw the body alone. `SB_SPOTS` names the points a story can click. `request_window(done)` draws the request window, waiting for a touch or granted. The dot-files README hero ports these, so keep them free of desk and bar globals.
- **`SCENES`** is the timeline, a list of `(name, seconds, function)`. The loop length is the sum of the seconds, and `start(name)` is where a scene begins.
- **Scene functions** get their start time `t0` and length `d` and add keyframes with absolute times: `key()` for any CSS property, `show()` and `swap()` for fades, `cut()` for an instant redraw (waybar and the panel never fade), `spring_open()` and `spring_close()` for windows, `tab_click()` for a tab switch, `cursor.go()` and `cursor.click()` for the pointer, `typed()` for typing.
- **`compile_css()`** turns each object's keys into one `@keyframes` over the whole loop, so percentages are never written by hand. Lengthening a scene or inserting one shifts everything after it.

## Following the real interface

The panel and the request window are drawn from `broker-board/ui.html` and `broker-dialog/ui.html` as they lay out in a browser: positions, sizes, wrapping and colors come from loading each page with a made-up snapshot and reading its boxes. The panel uses the colors it takes from the Platform waybar's `style.css` (see `internal/board/theme.go`). When either page changes, lay it out again the same way and update the numbers in `hero.py`.

What it shows follows the code, not a mock-up:

- The panel springs out of the click with the page's own curve, staggers its parts in once, switches tabs without blanking (the header and tabs redraw, the new body slides in 10px), and redraws in place after Revoke all, with the page's toast.
- The lamp is what `broker bar` prints (`internal/board/bar.go`), colored by `style.css`: a dim outline when nothing is granted, a pulsing amber key while an agent asks, a frost pie that drains with the soonest grant and the count of live secrets beside it. A count makes the module wider, so everything left of it moves over, as in waybar.
- The bar follows dot-files' `config.jsonc`. Its Nerd Font glyphs are outlines in `GLYPHS`, taken from JetBrainsMono Nerd Font, since an `<img>` loads no fonts.

## Adding a scene

1. Write `def scene_name(t0, d):` with its keys. If it needs a new object, draw it in a `build_*` function (or call `add(layer, svg)` from the scene) and give each moving part an `id`.
2. Add `("name", seconds, scene_name)` to `SCENES` where it belongs.
3. Run the script. It warns when an object ends the loop somewhere other than where it started, which shows up as a jump on restart. The `reset` scene snaps hidden objects back to their first frame on its own; anything still visible at the end needs its own way back.

## Palette and type

The constants at the top of `hero.py` are the desk's Nord colors (`INK`, `FG`, `ACCENT`, `CALM`, `WARN`, `ACT`), the bar's (`PILL`, `B_*`, from dot-files' `style.css`), the panel's (`SB_*`) and the request window's own (`DLG_*`, `CLAIM_*`, `TOUCH_*`). Text uses the `FONT` monospace stack, and the request window uses `SANS`. No web fonts and no images: GitHub would not load them.

## Previewing

Open `artwork/hero.svg` in a browser to watch the loop. To check one moment, freeze it:

```sh
python3 artwork/hero/hero.py --still 13.5 --out /tmp/frame.svg
```

`--still` stops every object where it is at that many seconds (the last key before it, not in between). Scene start times are printed on every run.

## Rules

- **CSS only.** GitHub shows the hero through an `<img>`, which runs CSS animations but no scripts and no external files. Do not add `<script>`, SMIL (`<animate>`), web fonts or linked images.
- **Animated properties stay out of inline styles.** An inline `style="opacity:0"` would beat the reduced-motion still. Pass `opacity` through `style=` on the helpers (they turn it into the `opacity` attribute) or write the attribute yourself. An element animated by its `id` cannot also run a class animation (the `id` rule wins), so put the class on a child, as the asking key does.
- **Reduced motion** shows a still of the open panel on the Live tab (`STILL` in `hero.py`). Keep that frame meaningful if you move scenes around.

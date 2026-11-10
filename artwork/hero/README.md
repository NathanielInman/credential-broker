# The README hero

`artwork/hero.svg` is generated. Edit `hero.py` and run it again:

```sh
python3 artwork/hero/hero.py
```

It needs Python 3 and nothing else.

## How the loop is built

The SVG is one loop in two acts: broker (an agent asks for a secret, the request window, a YubiKey touch, a redacted deploy), then Signal Box (the lamp counts grants, the panel opens out of it, a look at Asks, Revoke all, the panel folds back).

- **Objects** are drawn once by the `build_*` functions: the desk, the terminal, the request window, the YubiKey, the bar and its lamp, the panel, the tooltips, the cursor. Every part that moves has an `id`. Nothing in them is animated by itself.
- **`SCENES`** is the timeline, a list of `(name, seconds, function)`. The loop length is the sum of the seconds, and `start(name)` is where a scene begins.
- **Scene functions** get their start time `t0` and length `d` and add keyframes with absolute times: `key()` for any CSS property, `show()` and `swap()` for fades, `spring_open()` and `spring_close()` for panels, `rise()` for tiles, `cursor.go()` and `cursor.click()` for the pointer, `typed()` for terminal typing.
- **`compile_css()`** turns each object's keys into one `@keyframes` over the whole loop, so percentages are never written by hand. Lengthening a scene or inserting one shifts everything after it.

## Adding a scene

1. Write `def scene_name(t0, d):` with its keys. If it needs a new object, draw it in a `build_*` function (or call `add(layer, svg)` from the scene) and give each moving part an `id`.
2. Add `("name", seconds, scene_name)` to `SCENES` where it belongs.
3. Run the script. It warns when an object ends the loop somewhere other than where it started, which shows up as a jump on restart. The `reset` scene snaps hidden objects back to their first frame on its own; anything still visible at the end needs its own way back.

## Palette and type

The constants at the top of `hero.py` are the Nord colors Roundhouse and Switchyard use (`BAR`, `FG`, `DIM`, `ACCENT`, `CALM`, `WARN`, `ACT`), plus the request window's own colors from `broker-dialog/ui.html` (`DLG_*`, `CLAIM_*`, `TOUCH_*`). Text uses the `FONT` monospace stack, and the request window uses `SANS`. No web fonts and no images: GitHub would not load them.

## Previewing

Open `artwork/hero.svg` in a browser to watch the loop. To check one moment, freeze it:

```sh
python3 artwork/hero/hero.py --still 13.5 --out /tmp/frame.svg
```

`--still` stops every object where it is at that many seconds (the last key before it, not in between). Scene start times are printed on every run.

## Rules

- **CSS only.** GitHub shows the hero through an `<img>`, which runs CSS animations but no scripts and no external files. Do not add `<script>`, SMIL (`<animate>`), web fonts or linked images.
- **Animated properties stay out of inline styles.** An inline `style="opacity:0"` would beat the reduced-motion still. Pass `opacity` through `style=` on the helpers (they turn it into the `opacity` attribute) or write the attribute yourself.
- **Reduced motion** shows a still of the open panel on the Live tab (`STILL` in `hero.py`). Keep that frame meaningful if you move scenes around.

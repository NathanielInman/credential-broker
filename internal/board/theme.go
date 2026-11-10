package board

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Theme is the panel's look, read from the bar's stylesheet so the panel
// matches it. Empty fields keep the panel's built-in Nord defaults.
type Theme struct {
	Font   string `json:"font,omitempty"`
	Bar    string `json:"bar,omitempty"`
	Panel  string `json:"panel,omitempty"`
	Tile   string `json:"tile,omitempty"`
	FG     string `json:"fg,omitempty"`
	Dim    string `json:"dim,omitempty"`
	Line   string `json:"line,omitempty"`
	Accent string `json:"accent,omitempty"`
	Hover  string `json:"hover,omitempty"`
	// CSS is the user's own stylesheet, applied last.
	CSS string `json:"css,omitempty"`
}

var (
	cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssDefine  = regexp.MustCompile(`@define-color\s+([\w-]+)\s+([^;]+);`)
	cssRule    = regexp.MustCompile(`(?s)([^{}]+)\{([^{}]*)\}`)
	cssColor   = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b|rgba?\([^)]*\)|@[\w-]+`)
)

// WaybarTheme reads the font and colors out of a waybar style.css: the
// first font family on * or window#waybar, that window's background and
// text, the first border color on any rule, the first :hover background,
// and an accent from @define-color accent (or frost, or blue).
func WaybarTheme(css string) Theme {
	css = cssComment.ReplaceAllString(css, "")
	defs := map[string]string{}
	for _, m := range cssDefine.FindAllStringSubmatch(css, -1) {
		defs[m[1]] = strings.TrimSpace(m[2])
	}
	resolve := func(v string) string {
		for i := 0; i < 4 && strings.HasPrefix(v, "@"); i++ {
			v = defs[strings.TrimPrefix(v, "@")]
		}
		if c := cssColor.FindString(v); c != "" && !strings.HasPrefix(c, "@") {
			return c
		}
		return ""
	}
	var t Theme
	for _, m := range cssRule.FindAllStringSubmatch(css, -1) {
		// A rule's selector starts after any statement before it
		// (@define-color ...;, @import ...;).
		sel, body := m[1], m[2]
		if i := strings.LastIndexByte(sel, ';'); i >= 0 {
			sel = sel[i+1:]
		}
		sel = strings.TrimSpace(sel)
		props := declarations(body)
		isWindow := strings.Contains(sel, "window#waybar") && !strings.Contains(sel, ".")
		if t.Font == "" && (sel == "*" || isWindow) {
			if f := props["font-family"]; f != "" {
				t.Font = f
			}
		}
		if isWindow {
			for _, k := range []string{"background-color", "background"} {
				if c := resolve(props[k]); c != "" && t.Bar == "" {
					t.Bar = c
				}
			}
			if c := resolve(props["color"]); c != "" && t.FG == "" {
				t.FG = c
			}
		}
		if t.Line == "" {
			for _, k := range []string{"border-right", "border-left", "border-top", "border-bottom", "border", "border-color"} {
				if c := resolve(props[k]); c != "" {
					t.Line = c
					break
				}
			}
		}
		if t.Hover == "" && strings.Contains(sel, ":hover") {
			for _, k := range []string{"background-color", "background"} {
				if c := resolve(props[k]); c != "" {
					t.Hover = c
					break
				}
			}
		}
	}
	for _, k := range []string{"accent", "frost", "blue", "nord8"} {
		if c := resolve("@" + k); c != "" {
			t.Accent = c
			break
		}
	}
	if r, g, b, ok := parseColor(t.Bar); ok {
		t.Panel = mix(r, g, b, 255, 255, 255, 0.03)
		t.Tile = mix(r, g, b, 255, 255, 255, 0.07)
	}
	if r, g, b, ok := parseColor(t.FG); ok {
		if br, bg, bb, ok := parseColor(t.Bar); ok {
			t.Dim = mix(r, g, b, br, bg, bb, 0.42)
		}
	}
	return t
}

func declarations(body string) map[string]string {
	out := map[string]string{}
	for _, d := range strings.Split(body, ";") {
		k, v, ok := strings.Cut(d, ":")
		if !ok {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	return out
}

// parseColor reads #rgb, #rrggbb, #rrggbbaa, rgb() and rgba().
func parseColor(s string) (r, g, b int, ok bool) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "#") {
		h := s[1:]
		if len(h) == 3 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		if len(h) != 6 && len(h) != 8 {
			return 0, 0, 0, false
		}
		v, err := strconv.ParseUint(h[:6], 16, 32)
		if err != nil {
			return 0, 0, 0, false
		}
		return int(v >> 16), int(v >> 8 & 0xff), int(v & 0xff), true
	}
	if i := strings.IndexByte(s, '('); i > 0 && strings.HasSuffix(s, ")") {
		parts := strings.Split(s[i+1:len(s)-1], ",")
		if len(parts) < 3 {
			return 0, 0, 0, false
		}
		var c [3]int
		for j := 0; j < 3; j++ {
			f, err := strconv.ParseFloat(strings.TrimSpace(parts[j]), 64)
			if err != nil {
				return 0, 0, 0, false
			}
			c[j] = int(f)
		}
		return c[0], c[1], c[2], true
	}
	return 0, 0, 0, false
}

// mix is a + (b-a)*t as #rrggbb.
func mix(r, g, b, r2, g2, b2 int, t float64) string {
	f := func(a, b int) int { return int(float64(a) + float64(b-a)*t + 0.5) }
	return fmt.Sprintf("#%02x%02x%02x", f(r, r2), f(g, g2), f(b, b2))
}

// LoadTheme builds the panel's theme from cfg: the bar's stylesheet when
// theme.from is waybar, then the user's own board.css on top.
func LoadTheme(cfg *Config, home, userCSS string) Theme {
	var t Theme
	if cfg.Theme.From == "waybar" {
		if b, err := os.ReadFile(Expand(cfg.Theme.WaybarCSS, home)); err == nil {
			t = WaybarTheme(string(b))
		}
	}
	if b, err := os.ReadFile(userCSS); err == nil {
		t.CSS = string(b)
	}
	return t
}

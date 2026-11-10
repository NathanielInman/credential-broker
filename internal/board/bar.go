package board

import (
	"fmt"
	"math"
)

// BarOutput is one line of waybar's custom module protocol.
type BarOutput struct {
	Text       string `json:"text"`
	Class      string `json:"class"`
	Alt        string `json:"alt"`
	Tooltip    string `json:"tooltip"`
	Percentage int    `json:"percentage"`
}

// Nerd Font glyphs: an outline when quiet, a pie that drains with the
// soonest grant (md-circle-slice-1 to -8), a key for asks, a lock.
var nerdSlices = []string{"\U000F0A9E", "\U000F0A9F", "\U000F0AA0", "\U000F0AA1", "\U000F0AA2", "\U000F0AA3", "\U000F0AA4", "\U000F0AA5"}

const (
	nerdQuiet  = "\U000F0766"
	nerdKey    = "\U000F0306"
	nerdLocked = "\U000F033E"
)

var unicodeSlices = []string{"◔", "◔", "◑", "◑", "◕", "◕", "●", "●"}

// Bar renders the lamp for waybar. The text is the glyph and, with
// bar.count, how many secrets are live (or asks waiting); the class is the
// lamp level, for styling in waybar's own CSS.
func Bar(l Lamp, cfg *Config) BarOutput {
	slices, quiet, key, locked := nerdSlices, nerdQuiet, nerdKey, nerdLocked
	if cfg.Bar.Glyphs == "unicode" {
		slices, quiet, key, locked = unicodeSlices, "○", "◉", "◌"
	}
	out := BarOutput{Class: l.Level, Alt: l.Level, Tooltip: "Signal Box\n" + l.Tooltip}
	count := ""
	switch l.Level {
	case "quiet":
		out.Text = quiet
	case "asking", "missed":
		out.Text = key
	case "locked":
		out.Text = locked
	default:
		i := int(math.Ceil(l.Frac*8)) - 1
		out.Text = slices[min(max(i, 0), 7)]
		out.Percentage = int(math.Round(l.Frac * 100))
	}
	if cfg.Bar.Count != nil && *cfg.Bar.Count && l.Count > 0 && l.Level != "quiet" && l.Level != "locked" {
		count = fmt.Sprint(l.Count)
	}
	if l.Level == "missed" || l.Level == "asking" {
		count = ""
	}
	if count != "" {
		out.Text += " " + count
	}
	return out
}

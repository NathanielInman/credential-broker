// Package board is Signal Box: the lamp broker puts in a status bar and the
// panel that springs out of it. It works out what the lamp says and what
// the panel shows from what broker already keeps (the vault's names and
// tiers, the live grants, the audit log, and the requests open right now),
// and never sees a secret's value: nothing here can decrypt.
package board

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is board.toml. Every field is optional; an empty or missing file
// is a working setup.
type Config struct {
	Bar    BarConfig    `toml:"bar"`
	Board  PanelConfig  `toml:"board"`
	Theme  ThemeConfig  `toml:"theme"`
	Asks   AsksConfig   `toml:"asks"`
	Lock   LockConfig   `toml:"lock"`
	Expiry ExpiryConfig `toml:"expiry"`
	Vault  VaultConfig  `toml:"vault"`
	Bundle []Bundle     `toml:"bundle"`
}

type BarConfig struct {
	// Glyphs is "nerd" (Nerd Font icons, the default) or "unicode".
	Glyphs string `toml:"glyphs"`
	// Count shows how many secrets are live after the glyph.
	Count *bool `toml:"count"`
}

type PanelConfig struct {
	// Namespace is the bar's layer-shell namespace, which tells which
	// screen edge the bar is on.
	Namespace string `toml:"namespace"`
	// Linger is how long the hidden panel stays loaded, so the next open
	// is instant. After it, the panel's window process exits.
	Linger Duration `toml:"linger"`
}

type ThemeConfig struct {
	// From is "waybar" (font and colors from waybar's style.css) or "none".
	From string `toml:"from"`
	// WaybarCSS is the stylesheet read with from = "waybar".
	WaybarCSS string `toml:"waybar_css"`
}

type AsksConfig struct {
	// KeepMissed is how long a timed-out request stays on the Asks tab.
	KeepMissed Duration `toml:"keep_missed"`
}

type LockConfig struct {
	// OnScreenLock is "keep" or "revoke": revoke drops every grant when a
	// screen locker starts.
	OnScreenLock string `toml:"on_screen_lock"`
	// Lockers are the screen locker process names watched for.
	Lockers []string `toml:"lockers"`
	// NightAfter and NightUntil bound the hours (local, "HH:MM") in which
	// a live grant turns the lamp amber. Empty NightAfter turns it off.
	NightAfter string `toml:"night_after"`
	NightUntil string `toml:"night_until"`
}

type ExpiryConfig struct {
	// WarnBefore is how close to expiry a grant turns the lamp amber.
	WarnBefore Duration `toml:"warn_before"`
	// OnlyIfInUse limits that to grants used in the last ten minutes,
	// which an agent is about to find gone.
	OnlyIfInUse *bool `toml:"only_if_in_use"`
}

type VaultConfig struct {
	// StaleAfter flags secrets nothing has used for this long.
	StaleAfter Duration `toml:"stale_after"`
	// RotateAfter flags secrets whose value is older than this.
	RotateAfter Duration `toml:"rotate_after"`
}

// Bundle is a named set of secrets granted together with one touch.
type Bundle struct {
	Name string `toml:"name"`
	// Secrets are names or shell patterns (CLOUDFLARE_*).
	Secrets []string `toml:"secrets"`
	For     Duration `toml:"for"`
	// Key is the panel key that asks for the bundle.
	Key string `toml:"key"`
}

// Duration is a TOML duration string: Go's units plus d for days.
type Duration time.Duration

func (d *Duration) UnmarshalText(b []byte) error {
	v, err := ParseDuration(string(b))
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

func (d Duration) D() time.Duration { return time.Duration(d) }

// ParseDuration is time.ParseDuration plus a leading whole number of days
// ("30d", "1d12h").
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	var days time.Duration
	if i := strings.IndexByte(s, 'd'); i > 0 {
		n, err := strconv.Atoi(s[:i])
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		days = time.Duration(n) * 24 * time.Hour
		s = s[i+1:]
		if s == "" {
			return days, nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	return days + d, nil
}

// Defaults fills in every unset field.
func (c *Config) Defaults() {
	if c.Bar.Glyphs == "" {
		c.Bar.Glyphs = "nerd"
	}
	if c.Bar.Count == nil {
		t := true
		c.Bar.Count = &t
	}
	if c.Board.Namespace == "" {
		c.Board.Namespace = "waybar"
	}
	if c.Board.Linger == 0 {
		c.Board.Linger = Duration(10 * time.Minute)
	}
	if c.Theme.From == "" {
		c.Theme.From = "waybar"
	}
	if c.Theme.WaybarCSS == "" {
		c.Theme.WaybarCSS = "~/.config/waybar/style.css"
	}
	if c.Asks.KeepMissed == 0 {
		c.Asks.KeepMissed = Duration(24 * time.Hour)
	}
	if c.Lock.OnScreenLock == "" {
		c.Lock.OnScreenLock = "keep"
	}
	if len(c.Lock.Lockers) == 0 {
		c.Lock.Lockers = []string{"hyprlock", "swaylock", "gtklock", "waylock", "i3lock", "slock"}
	}
	if c.Lock.NightUntil == "" {
		c.Lock.NightUntil = "06:00"
	}
	if c.Expiry.WarnBefore == 0 {
		c.Expiry.WarnBefore = Duration(5 * time.Minute)
	}
	if c.Expiry.OnlyIfInUse == nil {
		t := true
		c.Expiry.OnlyIfInUse = &t
	}
	if c.Vault.StaleAfter == 0 {
		c.Vault.StaleAfter = Duration(14 * 24 * time.Hour)
	}
	if c.Vault.RotateAfter == 0 {
		c.Vault.RotateAfter = Duration(180 * 24 * time.Hour)
	}
	for i := range c.Bundle {
		if c.Bundle[i].For == 0 {
			c.Bundle[i].For = Duration(time.Hour)
		}
	}
}

func (c *Config) validate() error {
	switch c.Bar.Glyphs {
	case "nerd", "unicode":
	default:
		return fmt.Errorf("bar.glyphs is %q, want nerd or unicode", c.Bar.Glyphs)
	}
	switch c.Theme.From {
	case "waybar", "none":
	default:
		return fmt.Errorf("theme.from is %q, want waybar or none", c.Theme.From)
	}
	switch c.Lock.OnScreenLock {
	case "keep", "revoke":
	default:
		return fmt.Errorf("lock.on_screen_lock is %q, want keep or revoke", c.Lock.OnScreenLock)
	}
	for _, s := range []string{c.Lock.NightAfter, c.Lock.NightUntil} {
		if s == "" {
			continue
		}
		if _, err := clock(s); err != nil {
			return err
		}
	}
	keys := map[string]string{}
	for _, b := range c.Bundle {
		if b.Name == "" || len(b.Secrets) == 0 {
			return errors.New("every [[bundle]] needs a name and secrets")
		}
		for _, pat := range b.Secrets {
			if _, err := path.Match(pat, ""); err != nil {
				return fmt.Errorf("bundle %s: bad pattern %q", b.Name, pat)
			}
		}
		if b.Key != "" {
			if len(b.Key) != 1 || strings.ContainsAny(b.Key, "1234rReEgGjk/ ") {
				return fmt.Errorf("bundle %s: key %q is taken or not one character", b.Name, b.Key)
			}
			if other, ok := keys[b.Key]; ok {
				return fmt.Errorf("bundles %s and %s both use key %q", other, b.Name, b.Key)
			}
			keys[b.Key] = b.Name
		}
	}
	return nil
}

// clock parses "HH:MM" as minutes after midnight.
func clock(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("invalid time %q, want HH:MM", s)
	}
	return t.Hour()*60 + t.Minute(), nil
}

// LoadConfig reads board.toml at file. A missing file is the defaults.
func LoadConfig(file string) (*Config, error) {
	var c Config
	data, err := os.ReadFile(file)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		md, err := toml.Decode(string(data), &c)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		if extra := md.Undecoded(); len(extra) > 0 {
			return nil, fmt.Errorf("%s: unknown setting %s", file, extra[0])
		}
	}
	c.Defaults()
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &c, nil
}

// Expand resolves a leading ~/ against home.
func Expand(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// Members resolves b's patterns against the vault's names, in vault order.
func (b Bundle) Members(names []string) []string {
	var out []string
	for _, n := range names {
		for _, pat := range b.Secrets {
			if ok, _ := path.Match(pat, n); ok {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

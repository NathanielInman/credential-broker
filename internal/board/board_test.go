package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

func TestParseDuration(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"30d": 30 * 24 * time.Hour, "1d12h": 36 * time.Hour, "90m": 90 * time.Minute, "5m": 5 * time.Minute,
	} {
		got, err := ParseDuration(in)
		if err != nil || got != want {
			t.Errorf("ParseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "d", "xd", "3 days"} {
		if _, err := ParseDuration(bad); err == nil {
			t.Errorf("ParseDuration(%q) succeeded", bad)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	c, err := LoadConfig(filepath.Join(dir, "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Bar.Glyphs != "nerd" || c.Lock.OnScreenLock != "keep" || c.Vault.StaleAfter.D() != 14*24*time.Hour {
		t.Fatalf("defaults: %+v", c)
	}

	file := filepath.Join(dir, "board.toml")
	os.WriteFile(file, []byte(`
[lock]
on_screen_lock = "revoke"
night_after = "01:00"

[[bundle]]
name = "cloudflare"
secrets = ["CLOUDFLARE_*"]
for = "2h"
key = "c"
`), 0o600)
	c, err = LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	if c.Lock.OnScreenLock != "revoke" || c.Bundle[0].For.D() != 2*time.Hour {
		t.Fatalf("got %+v", c)
	}
	got := c.Bundle[0].Members([]string{"CLOUDFLARE_API_TOKEN", "JIRA_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID"})
	if strings.Join(got, ",") != "CLOUDFLARE_API_TOKEN,CLOUDFLARE_ACCOUNT_ID" {
		t.Fatalf("members %v", got)
	}

	for _, bad := range []string{
		"[bar]\nglyphs = \"emoji\"",
		"[lock]\non_screen_lock = \"maybe\"",
		"[lock]\nnight_after = \"25:99\"",
		"[[bundle]]\nname = \"x\"\nsecrets = [\"A\"]\nkey = \"r\"",
		"[nope]\nx = 1",
	} {
		os.WriteFile(file, []byte(bad), 0o600)
		if _, err := LoadConfig(file); err == nil {
			t.Errorf("LoadConfig accepted %q", bad)
		}
	}
}

const monitors = `[
 {"name":"DP-3","x":0,"y":0,"width":2560,"height":1440,"scale":1.0,"transform":0,"reserved":[0,0,0,24],"focused":false},
 {"name":"DP-2","x":2560,"y":0,"width":2560,"height":1440,"scale":1.0,"transform":0,"reserved":[0,0,0,24],"focused":true}
]`

func TestComputeAnchor(t *testing.T) {
	layers := `{"DP-2":{"levels":{"2":[{"x":2560,"y":1416,"w":2560,"h":24,"namespace":"waybar"}]}}}`
	a, err := ComputeAnchor([]byte(`{"x":4870,"y":1428}`), []byte(monitors), []byte(layers), "waybar", false)
	if err != nil {
		t.Fatal(err)
	}
	if a.Monitor != "DP-2" || a.Edge != "bottom" || a.X != 2310 || a.Reserved[3] != 24 {
		t.Fatalf("got %+v", a)
	}
	left := `{"DP-3":{"levels":{"2":[{"x":0,"y":0,"w":40,"h":1440,"namespace":"waybar"}]}}}`
	if a, _ := ComputeAnchor([]byte(`{"x":10,"y":700}`), []byte(monitors), []byte(left), "waybar", false); a.Edge != "left" {
		t.Fatalf("left bar read as %s", a.Edge)
	}
	top := `[{"name":"eDP-1","x":0,"y":0,"width":2880,"height":1800,"scale":2.0,"reserved":[0,32,0,0]}]`
	a, _ = ComputeAnchor([]byte(`{"x":700,"y":5}`), []byte(top), []byte(`{}`), "waybar", false)
	if a.Width != 1440 || a.Edge != "top" {
		t.Fatalf("scaled top bar: %+v", a)
	}
	a, _ = ComputeAnchor(nil, []byte(monitors), nil, "waybar", true)
	if a.Monitor != "DP-2" || !a.Center {
		t.Fatalf("center: %+v", a)
	}
}

func TestWaybarTheme(t *testing.T) {
	css := `
/* Nord */
@define-color frost #88C0D0;
* { font-family: "PragmataPro Liga", monospace; font-size: 18px; }
window#waybar { background: #282A2E; color: #C5C8C6; }
window#waybar.hidden { opacity: .2; }
#clock { padding: 0 6px; border-right: 1px solid #4c566a; }
#clock:hover { background: rgba(76, 86, 106, 0.6); }
`
	th := WaybarTheme(css)
	if th.Font != `"PragmataPro Liga", monospace` || th.Bar != "#282A2E" || th.FG != "#C5C8C6" ||
		th.Line != "#4c566a" || th.Accent != "#88C0D0" || !strings.HasPrefix(th.Hover, "rgba(") {
		t.Fatalf("got %+v", th)
	}
	if th.Panel == "" || th.Dim == "" {
		t.Fatalf("derived colors missing: %+v", th)
	}
}

func TestAsks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "asks")
	now := time.Now()
	WriteAsk(dir, Ask{Names: []string{"A"}, PID: 10, Opened: now, Deadline: now.Add(time.Minute)})
	WriteAsk(dir, Ask{Names: []string{"B"}, PID: 11, Opened: now, Deadline: now.Add(-time.Hour)})
	WriteAsk(dir, Ask{Names: []string{"C"}, PID: 12, Opened: now, Deadline: now.Add(time.Minute)})
	got := ReadAsks(dir, now, func(pid int) bool { return pid != 12 })
	if len(got) != 1 || got[0].Names[0] != "A" {
		t.Fatalf("got %+v", got)
	}
	if _, err := os.Stat(AskFile(dir, 11)); !os.IsNotExist(err) {
		t.Fatal("expired ask left behind")
	}
	RemoveAsk(dir, 10)
	if len(ReadAsks(dir, now, nil)) != 0 {
		t.Fatal("removed ask still read")
	}
}

func entry(ts time.Time, action audit.Action, ok bool, names ...string) audit.Entry {
	return audit.Entry{TS: ts, Action: action, OK: ok, Names: names, Ancestry: []string{"broker", "zsh", "claude", "t3code"}, Cwd: "/home/u/proj"}
}

func TestBuild(t *testing.T) {
	now := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	created := now.Add(-20 * 24 * time.Hour)
	secrets := []vault.Secret{
		{Name: "JIRA_API_TOKEN", Tier: vault.Private, Description: "Jira", Created: created, Updated: created},
		{Name: "SONARR_API_KEY", Tier: vault.Private, Description: "Sonarr", Created: created, Updated: created},
		{Name: "RADARR_API_KEY", Tier: vault.Private, Description: "Radarr", Created: created, Updated: created},
		{Name: "FIGMA_TOKEN", Tier: vault.Private, Description: "Figma", Created: created, Updated: created},
		{Name: "SENTRY_AUTH_TOKEN", Tier: vault.Private, Created: created, Updated: created},
		{Name: "KEYSTORE_PASSWORD", Tier: vault.Public, Description: "pw", Created: created, Updated: created},
	}
	req := entry(now.Add(-40*time.Minute), audit.Request, true, "JIRA_API_TOKEN")
	req.For, req.Reason, req.Via = "1h", "QA pass", "desktop"
	bundle := entry(now.Add(-2*time.Hour), audit.Request, true, "RADARR_API_KEY", "SONARR_API_KEY")
	bundle.For = "9h"
	bundle.Ancestry = []string{"broker", "broker", "systemd"}
	miss := entry(now.Add(-3*time.Hour), audit.Request, false, "FIGMA_TOKEN")
	miss.Error, miss.For, miss.Reason = "request_timeout", "1h", "export frames"
	miss2 := miss
	miss2.TS = now.Add(-150 * time.Minute)
	answered := entry(now.Add(-5*time.Hour), audit.Request, false, "SENTRY_AUTH_TOKEN")
	answered.Error, answered.For = "request_timeout", "1h"
	answeredLater := entry(now.Add(-4*time.Hour), audit.Request, true, "SENTRY_AUTH_TOKEN")
	answeredLater.For = "1h"
	failed := entry(now.Add(-9*time.Minute), audit.Exec, true, "JIRA_API_TOKEN")
	failed.PID, failed.Cmd = 77, "jira"
	status := 1
	failedExit := entry(now.Add(-9*time.Minute+time.Second), audit.Exit, true, "JIRA_API_TOKEN")
	failedExit.PID, failedExit.Status, failedExit.Took = 77, &status, "1s"
	log := []audit.Entry{
		answered, answeredLater, miss, bundle, miss2, req,
		entry(now.Add(-30*time.Minute), audit.Exec, true, "JIRA_API_TOKEN"),
		entry(now.Add(-20*time.Minute), audit.Exec, true, "JIRA_API_TOKEN"),
		failed, failedExit,
		entry(now.Add(-6*time.Minute), audit.Exec, true, "SONARR_API_KEY"),
	}
	cfg := &Config{Bundle: []Bundle{{Name: "homelab", Secrets: []string{"*ARR_API_KEY"}, Key: "h"}}}
	cfg.Defaults()
	in := Input{
		Now: now, Home: "/home/u", Secrets: secrets, Log: log, Config: cfg,
		Grants: []GrantIn{
			{Name: "JIRA_API_TOKEN", Expires: now.Add(20 * time.Minute)},
			{Name: "SONARR_API_KEY", Expires: now.Add(7 * time.Hour)},
			{Name: "RADARR_API_KEY", Expires: now.Add(7 * time.Hour)},
		},
	}
	s := Build(in)

	if len(s.Grants) != 2 {
		t.Fatalf("grants: %+v", s.Grants)
	}
	g := s.Grants[0]
	if g.Names[0] != "JIRA_API_TOKEN" || g.Who != "claude" || g.Reason != "QA pass" || g.Runs != 3 || g.Total != 3600 || g.Cwd != "~/proj" {
		t.Fatalf("jira row: %+v", g)
	}
	if b := s.Grants[1]; b.Bundle != "homelab" || b.Who != "Signal Box" || len(b.Names) != 2 {
		t.Fatalf("bundle row: %+v", b)
	}
	if len(s.Missed) != 1 || s.Missed[0].Names[0] != "FIGMA_TOKEN" || s.Missed[0].Retries != 1 {
		t.Fatalf("missed: %+v", s.Missed)
	}
	if s.Lamp.Level != "missed" || s.Lamp.Count != 3 {
		t.Fatalf("lamp: %+v", s.Lamp)
	}

	in.Dismissed = Dismissed{s.Missed[0].Key: true}
	s = Build(in)
	if len(s.Missed) != 0 || s.Lamp.Level != "live" {
		t.Fatalf("after dismiss: %+v %+v", s.Missed, s.Lamp)
	}
	if !strings.Contains(s.Lamp.Tooltip, "3 secrets live in 2 grants") || !strings.Contains(s.Lamp.Tooltip, "JIRA_API_TOKEN ends in 20m") {
		t.Fatalf("tooltip %q", s.Lamp.Tooltip)
	}

	// Ending: under five minutes left and used in the last ten.
	in.Grants[0].Expires = now.Add(3 * time.Minute)
	if s = Build(in); s.Lamp.Level != "ending" {
		t.Fatalf("want ending, got %+v", s.Lamp)
	}
	in.Asks = []Ask{{Names: []string{"FIGMA_TOKEN"}, Ancestry: []string{"broker", "claude"}, Deadline: now.Add(90 * time.Second)}}
	if s = Build(in); s.Lamp.Level != "asking" || !strings.Contains(s.Lamp.Tooltip, "claude is asking for FIGMA_TOKEN · 1:30 left") {
		t.Fatalf("asking: %+v", s.Lamp)
	}

	var run *Episode
	for i, e := range s.Register.Episodes {
		if e.Kind == "run" && e.Names[0] == "JIRA_API_TOKEN" {
			run = &s.Register.Episodes[i]
		}
	}
	if run == nil || run.Runs != 3 || run.Failed != 1 || run.Why != "jira exit 1" {
		t.Fatalf("run episode: %+v", run)
	}

	flags := map[string]string{}
	for _, r := range s.Vault.Rows {
		for _, f := range r.Flags {
			flags[r.Name] += f.Text + ";"
		}
	}
	if !strings.Contains(flags["FIGMA_TOKEN"], "never used") || !strings.Contains(flags["SENTRY_AUTH_TOKEN"], "no description") ||
		!strings.Contains(flags["KEYSTORE_PASSWORD"], "public, but named like a secret") || flags["JIRA_API_TOKEN"] != "" {
		t.Fatalf("flags: %v", flags)
	}

	in.Grants, in.Asks, in.Dismissed = nil, nil, Dismissed{"x": true}
	in.Log = nil
	in.Locked = &Locked{At: now.Add(-time.Hour), Names: []string{"A", "B"}}
	if s = Build(in); s.Lamp.Level != "locked" {
		t.Fatalf("locked: %+v", s.Lamp)
	}
}

func TestNight(t *testing.T) {
	c := &Config{Lock: LockConfig{NightAfter: "23:30"}}
	c.Defaults()
	at := func(h, m int) time.Time { return time.Date(2026, 1, 1, h, m, 0, 0, time.Local) }
	for _, tc := range []struct {
		t    time.Time
		want bool
	}{{at(23, 45), true}, {at(3, 0), true}, {at(6, 0), false}, {at(12, 0), false}} {
		if got := night(c, tc.t); got != tc.want {
			t.Errorf("night at %s = %v", tc.t.Format("15:04"), got)
		}
	}
}

func TestBar(t *testing.T) {
	c := &Config{}
	c.Defaults()
	out := Bar(Lamp{Level: "live", Count: 7, Frac: 0.5}, c)
	if out.Text != nerdSlices[3]+" 7" || out.Class != "live" || out.Percentage != 50 {
		t.Fatalf("live: %+v", out)
	}
	if out := Bar(Lamp{Level: "quiet"}, c); out.Text != nerdQuiet {
		t.Fatalf("quiet: %+v", out)
	}
	c.Bar.Glyphs = "unicode"
	if out := Bar(Lamp{Level: "asking", Count: 2}, c); out.Text != "◉" || out.Class != "asking" {
		t.Fatalf("asking: %+v", out)
	}
}

func TestWho(t *testing.T) {
	for _, tc := range []struct {
		chain  []string
		action audit.Action
		want   string
	}{
		{[]string{"broker", "bash", "zsh", "claude", "t3code"}, audit.Request, "claude"},
		{[]string{"broker", "broker", "systemd"}, audit.Request, "Signal Box"},
		{[]string{"broker", "zsh", "alacritty"}, audit.Grant, "you"},
		{[]string{"broker", "make", "bash"}, audit.Exec, "make"},
	} {
		if got := Who(tc.chain, tc.action); got != tc.want {
			t.Errorf("Who(%v) = %q, want %q", tc.chain, got, tc.want)
		}
	}
}

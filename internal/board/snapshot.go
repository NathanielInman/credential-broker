package board

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/vault"
)

// Input is everything a Snapshot is worked out from. The caller gathers
// it; building is pure, so it can be tested with made up logs.
type Input struct {
	Now    time.Time
	Host   string
	Home   string
	Zone   string
	Device string
	// Secrets are the vault's secrets (names, tiers, descriptions).
	Secrets []vault.Secret
	// Grants are the grants still in place: live ones, and changed ones
	// exec refuses because the secret changed since.
	Grants []GrantIn
	// Log is the audit log, oldest first.
	Log       []audit.Entry
	Asks      []Ask
	Dismissed Dismissed
	Locked    *Locked
	Config    *Config
	// Alive reports whether a broker pid is still running, for runs the
	// log has no exit for yet. Nil treats them all as finished.
	Alive func(pid int) bool
	// Siblings maps "switchyard" and "roundhouse" to their lamp level,
	// for the ones that are running.
	Siblings map[string]string
}

// GrantIn is one grant in place.
type GrantIn struct {
	Name    string
	Expires time.Time
	Changed bool
}

// Snapshot is what the lamp and the panel show.
type Snapshot struct {
	Now      time.Time         `json:"now"`
	Host     string            `json:"host"`
	Zone     string            `json:"zone"`
	Device   string            `json:"device,omitempty"`
	Lamp     Lamp              `json:"lamp"`
	Grants   []GrantRow        `json:"grants"`
	Asks     []AskRow          `json:"asks"`
	Missed   []MissedRow       `json:"missed"`
	Today    Today             `json:"today"`
	Register Register          `json:"register"`
	Vault    VaultView         `json:"vault"`
	Bundles  []BundleRow       `json:"bundles"`
	Siblings map[string]string `json:"siblings,omitempty"`
}

// Lamp is the bar icon's state. Level is one of quiet, live, ending,
// night, asking, missed, locked.
type Lamp struct {
	Level   string  `json:"level"`
	Count   int     `json:"count"`
	Frac    float64 `json:"frac"`
	Tooltip string  `json:"tooltip"`
}

type GrantRow struct {
	ID       string     `json:"id"`
	Names    []string   `json:"names"`
	Bundle   string     `json:"bundle,omitempty"`
	Granted  time.Time  `json:"granted"`
	Expires  time.Time  `json:"expires"`
	Total    int64      `json:"total"`
	Who      string     `json:"who"`
	Cwd      string     `json:"cwd,omitempty"`
	Reason   string     `json:"reason,omitempty"`
	Via      string     `json:"via,omitempty"`
	Runs     int        `json:"runs"`
	LastUsed *time.Time `json:"last_used,omitempty"`
	Spark    []int      `json:"spark"`
	Changed  bool       `json:"changed,omitempty"`
}

type AskRow struct {
	Names    []string  `json:"names"`
	For      string    `json:"for"`
	Reason   string    `json:"reason,omitempty"`
	Who      string    `json:"who"`
	Chain    string    `json:"chain"`
	Cwd      string    `json:"cwd,omitempty"`
	Cmd      string    `json:"cmd,omitempty"`
	Deadline time.Time `json:"deadline"`
	Queued   bool      `json:"queued,omitempty"`
	Extend   bool      `json:"extend,omitempty"`
}

type MissedRow struct {
	Key     string    `json:"key"`
	TS      time.Time `json:"ts"`
	Names   []string  `json:"names"`
	For     string    `json:"for"`
	Reason  string    `json:"reason,omitempty"`
	Who     string    `json:"who"`
	Cwd     string    `json:"cwd,omitempty"`
	Retries int       `json:"retries"`
}

type Today struct {
	Answered int `json:"answered"`
	Missed   int `json:"missed"`
	Runs     int `json:"runs"`
}

type Register struct {
	Strip    []StripMark `json:"strip"`
	Episodes []Episode   `json:"episodes"`
}

// StripMark is one tick on the 24 hour strip: T is Unix seconds.
type StripMark struct {
	T int64  `json:"t"`
	K string `json:"k"`
}

// Episode is a run of related log lines read as one: the runs one agent
// made with the same secrets in the same directory, or a single grant,
// miss, denial or revoke. Kind is run, grant, missed, denied, revoke,
// lock, or change.
type Episode struct {
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
	Kind   string    `json:"kind"`
	Names  []string  `json:"names"`
	Who    string    `json:"who,omitempty"`
	Cwd    string    `json:"cwd,omitempty"`
	Cmd    string    `json:"cmd,omitempty"`
	Runs   int       `json:"runs,omitempty"`
	Failed int       `json:"failed,omitempty"`
	Why    string    `json:"why,omitempty"`
	Detail string    `json:"detail,omitempty"`
	Live   bool      `json:"live,omitempty"`
}

type VaultView struct {
	Total   int        `json:"total"`
	Private int        `json:"private"`
	Public  int        `json:"public"`
	Flagged int        `json:"flagged"`
	Stale   int        `json:"stale"`
	Rows    []VaultRow `json:"rows"`
}

type VaultRow struct {
	Name        string     `json:"name"`
	Tier        string     `json:"tier"`
	Description string     `json:"description,omitempty"`
	Runs        int        `json:"runs"`
	LastUsed    *time.Time `json:"last_used,omitempty"`
	Spark       []int      `json:"spark"`
	Live        *time.Time `json:"live,omitempty"`
	Flags       []Flag     `json:"flags,omitempty"`
}

// Flag is a note on a secret: Level is info, warn, or bad.
type Flag struct {
	Level string `json:"level"`
	Text  string `json:"text"`
}

type BundleRow struct {
	Name  string   `json:"name"`
	Names []string `json:"names"`
	For   int64    `json:"for"`
	Key   string   `json:"key,omitempty"`
	Live  bool     `json:"live"`
}

// knownAgents are the process names read as "who" in a process chain.
var knownAgents = []string{"claude", "codex", "opencode", "gemini", "cursor-agent", "cursor", "aider", "goose", "amp", "crush", "copilot", "t3code"}

// Who names the requester in a process chain as people know it: the
// first agent in it, "you" for a human command, or "Signal Box" for one
// the panel ran.
func Who(ancestry []string, action audit.Action) string {
	chain := ancestry
	if len(chain) > 0 && chain[0] == "broker" {
		chain = chain[1:]
	}
	if len(chain) > 0 && chain[0] == "broker" {
		return "Signal Box"
	}
	for _, a := range chain {
		for _, k := range knownAgents {
			if a == k {
				return a
			}
		}
	}
	if action == audit.Grant || action == audit.Revoke || action == audit.Set || action == audit.Rm {
		return "you"
	}
	if len(chain) > 0 {
		return chain[0]
	}
	return "unknown"
}

// Short replaces home with ~ at the start of p.
func Short(p, home string) string {
	if home != "" && (p == home || strings.HasPrefix(p, home+"/")) {
		return "~" + p[len(home):]
	}
	return p
}

func chain(ancestry []string) string {
	a := ancestry
	if len(a) > 0 && a[0] == "broker" {
		a = a[1:]
	}
	if len(a) > 4 {
		a = a[:4]
	}
	return strings.Join(a, " < ")
}

func covers(e audit.Entry, name string) bool { return slices.Contains(e.Names, name) }

func granting(e audit.Entry) bool {
	return e.OK && (e.Action == audit.Grant || e.Action == audit.Request) && e.For != "" && len(e.Names) > 0
}

// Build works out the snapshot.
func Build(in Input) Snapshot {
	cfg := in.Config
	if cfg == nil {
		cfg = &Config{}
		cfg.Defaults()
	}
	now := in.Now
	s := Snapshot{Now: now, Host: in.Host, Zone: in.Zone, Device: in.Device, Siblings: in.Siblings}
	names := make([]string, 0, len(in.Secrets))
	for _, sec := range in.Secrets {
		names = append(names, sec.Name)
	}
	s.Grants = grantRows(in, names)
	s.Asks = askRows(in)
	s.Missed = missedRows(in)
	s.Register, s.Today = register(in)
	s.Vault = vaultView(in)
	for _, b := range cfg.Bundle {
		members := b.Members(names)
		if len(members) == 0 {
			continue
		}
		live := true
		for _, m := range members {
			if !liveIn(s.Grants, m) {
				live = false
			}
		}
		s.Bundles = append(s.Bundles, BundleRow{Name: b.Name, Names: members, For: int64(b.For.D() / time.Second), Key: b.Key, Live: live})
	}
	s.Lamp = lamp(in, s)
	// Empty lists, not null, for the page.
	if s.Grants == nil {
		s.Grants = []GrantRow{}
	}
	if s.Asks == nil {
		s.Asks = []AskRow{}
	}
	if s.Missed == nil {
		s.Missed = []MissedRow{}
	}
	if s.Bundles == nil {
		s.Bundles = []BundleRow{}
	}
	if s.Register.Strip == nil {
		s.Register.Strip = []StripMark{}
	}
	if s.Register.Episodes == nil {
		s.Register.Episodes = []Episode{}
	}
	if s.Vault.Rows == nil {
		s.Vault.Rows = []VaultRow{}
	}
	return s
}

func liveIn(rows []GrantRow, name string) bool {
	for _, r := range rows {
		if !r.Changed && slices.Contains(r.Names, name) {
			return true
		}
	}
	return false
}

func grantRows(in Input, vaultNames []string) []GrantRow {
	type group struct {
		src   int
		names []string
		in    []GrantIn
	}
	groups := map[string]*group{}
	var order []string
	for _, g := range in.Grants {
		src := -1
		for i := len(in.Log) - 1; i >= 0; i-- {
			if e := in.Log[i]; granting(e) && covers(e, g.Name) && !e.TS.After(in.Now) {
				src = i
				break
			}
		}
		key := g.Name
		if src >= 0 {
			key = fmt.Sprintf("%d", src)
		}
		if g.Changed {
			key += ":changed"
		}
		gr, ok := groups[key]
		if !ok {
			gr = &group{src: src}
			groups[key] = gr
			order = append(order, key)
		}
		gr.names = append(gr.names, g.Name)
		gr.in = append(gr.in, g)
	}
	var rows []GrantRow
	for _, key := range order {
		gr := groups[key]
		sort.Strings(gr.names)
		r := GrantRow{ID: key, Names: gr.names, Who: "unknown", Changed: gr.in[0].Changed}
		for _, g := range gr.in {
			if r.Expires.IsZero() || (!g.Expires.IsZero() && g.Expires.Before(r.Expires)) {
				r.Expires = g.Expires
			}
		}
		if gr.src >= 0 {
			e := in.Log[gr.src]
			r.Granted = e.TS
			r.Who = Who(e.Ancestry, e.Action)
			r.Cwd = Short(e.Cwd, in.Home)
			r.Reason = e.Reason
			r.Via = e.Via
			if ttl, err := time.ParseDuration(e.For); err == nil {
				r.Total = int64(ttl / time.Second)
			}
			r.ID = e.TS.Format(time.RFC3339Nano)
		} else {
			r.Granted = in.Now
			r.Total = int64(r.Expires.Sub(in.Now) / time.Second)
		}
		if r.Total <= 0 {
			r.Total = 1
		}
		r.Spark = make([]int, 12)
		span := in.Now.Sub(r.Granted)
		for _, e := range in.Log {
			if e.Action != audit.Exec || !e.OK || e.TS.Before(r.Granted) {
				continue
			}
			hit := false
			for _, n := range r.Names {
				if covers(e, n) {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
			r.Runs++
			t := e.TS
			r.LastUsed = &t
			if span > 0 {
				b := int(float64(e.TS.Sub(r.Granted)) / float64(span) * 12)
				r.Spark[min(max(b, 0), 11)]++
			}
		}
		if cfgBundle := bundleOf(in.Config, vaultNames, r.Names); cfgBundle != "" {
			r.Bundle = cfgBundle
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Changed != rows[j].Changed {
			return !rows[i].Changed
		}
		return rows[i].Expires.Before(rows[j].Expires)
	})
	return rows
}

func bundleOf(cfg *Config, vaultNames, names []string) string {
	if cfg == nil {
		return ""
	}
	for _, b := range cfg.Bundle {
		m := b.Members(vaultNames)
		if len(m) == len(names) && len(m) > 1 {
			sorted := slices.Clone(m)
			sort.Strings(sorted)
			if slices.Equal(sorted, names) {
				return b.Name
			}
		}
	}
	return ""
}

func askRows(in Input) []AskRow {
	var out []AskRow
	for _, a := range in.Asks {
		out = append(out, AskRow{
			Names: a.Names, For: a.For, Reason: a.Reason,
			Who: Who(a.Ancestry, audit.Request), Chain: chain(a.Ancestry),
			Cwd: Short(a.Cwd, in.Home), Cmd: a.Cmd, Deadline: a.Deadline,
			Queued: a.Queued, Extend: a.Extend,
		})
	}
	return out
}

func missedRows(in Input) []MissedRow {
	keep := in.Config.Asks.KeepMissed.D()
	if keep <= 0 {
		keep = 24 * time.Hour
	}
	cutoff := in.Now.Add(-keep)
	// The latest timeout for each set of names, with how many came before.
	type miss struct {
		idx     int
		retries int
	}
	byNames := map[string]*miss{}
	var keys []string
	for i, e := range in.Log {
		if e.Action != audit.Request || e.Error != "request_timeout" || e.TS.Before(cutoff) || len(e.Names) == 0 {
			continue
		}
		k := strings.Join(e.Names, ",")
		if m, ok := byNames[k]; ok {
			m.idx = i
			m.retries++
			continue
		}
		byNames[k] = &miss{idx: i}
		keys = append(keys, k)
	}
	var out []MissedRow
	for _, k := range keys {
		m := byNames[k]
		e := in.Log[m.idx]
		key := e.TS.UTC().Format(time.RFC3339Nano)
		if in.Dismissed[key] {
			continue
		}
		// Answered since: every name was granted after the timeout.
		answered := true
		for _, n := range e.Names {
			got := false
			for _, later := range in.Log[m.idx+1:] {
				if granting(later) && covers(later, n) {
					got = true
					break
				}
			}
			if !got {
				answered = false
				break
			}
		}
		if answered {
			continue
		}
		out = append(out, MissedRow{
			Key: key, TS: e.TS, Names: e.Names, For: e.For, Reason: e.Reason,
			Who: Who(e.Ancestry, e.Action), Cwd: Short(e.Cwd, in.Home), Retries: m.retries,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TS.After(out[j].TS) })
	return out
}

// episodeGap is how far apart two runs may be and still read as one
// episode.
const episodeGap = 20 * time.Minute

func register(in Input) (Register, Today) {
	var reg Register
	var today Today
	since := in.Now.Add(-24 * time.Hour)
	midnight := time.Date(in.Now.Year(), in.Now.Month(), in.Now.Day(), 0, 0, 0, 0, in.Now.Location())

	// Each exec's exit, by index.
	exitOf := map[int]audit.Entry{}
	open := map[int]int{}
	for i, e := range in.Log {
		switch {
		case e.Action == audit.Exec && e.OK:
			open[e.PID] = i
		case e.Action == audit.Exit:
			if j, ok := open[e.PID]; ok {
				exitOf[j] = e
				delete(open, e.PID)
			}
		}
	}

	type runKey struct{ names, cwd, who string }
	current := map[runKey]*Episode{}
	var eps []*Episode
	for i, e := range in.Log {
		if e.TS.Before(since) {
			continue
		}
		local := e.TS.In(in.Now.Location())
		who := Who(e.Ancestry, e.Action)
		switch {
		case e.Action == audit.Exec:
			if e.OK {
				reg.Strip = append(reg.Strip, StripMark{e.TS.Unix(), "run"})
				if !local.Before(midnight) {
					today.Runs++
				}
			}
			k := runKey{strings.Join(e.Names, ","), e.Cwd, who}
			ep := current[k]
			if ep == nil || e.TS.Sub(ep.End) > episodeGap {
				ep = &Episode{Start: e.TS, End: e.TS, Kind: "run", Names: e.Names, Who: who, Cwd: Short(e.Cwd, in.Home), Cmd: e.Cmd}
				current[k] = ep
				eps = append(eps, ep)
			}
			ep.End = e.TS
			if e.OK {
				ep.Runs++
				if x, ok := exitOf[i]; ok {
					if x.Status != nil && *x.Status != 0 {
						ep.Failed++
						ep.Why = fmt.Sprintf("%s exit %d", e.Cmd, *x.Status)
					}
					if d, err := time.ParseDuration(x.Took); err == nil && x.TS.After(ep.End) {
						ep.End = e.TS.Add(d)
					}
				} else if in.Alive != nil && in.Alive(e.PID) {
					ep.Live = true
				}
			} else {
				ep.Failed++
				ep.Why = e.Error
			}
		case granting(e):
			reg.Strip = append(reg.Strip, StripMark{e.TS.Unix(), "grant"})
			if e.Action == audit.Request && !local.Before(midnight) {
				today.Answered++
			}
			d := "for " + e.For
			if e.Via != "" {
				d += " via " + e.Via
			}
			eps = append(eps, &Episode{Start: e.TS, End: e.TS, Kind: "grant", Names: e.Names, Who: who, Cwd: Short(e.Cwd, in.Home), Detail: d, Why: e.Reason})
		case e.Action == audit.Request && e.Error == "request_timeout":
			reg.Strip = append(reg.Strip, StripMark{e.TS.Unix(), "missed"})
			if !local.Before(midnight) {
				today.Missed++
			}
			eps = append(eps, &Episode{Start: e.TS, End: e.TS, Kind: "missed", Names: e.Names, Who: who, Cwd: Short(e.Cwd, in.Home), Why: e.Reason})
		case e.Action == audit.Request && e.Error == "denied":
			reg.Strip = append(reg.Strip, StripMark{e.TS.Unix(), "denied"})
			eps = append(eps, &Episode{Start: e.TS, End: e.TS, Kind: "denied", Names: e.Names, Who: who, Cwd: Short(e.Cwd, in.Home), Why: e.Reason})
		case e.Action == audit.Revoke && e.OK && len(e.Names) > 0:
			reg.Strip = append(reg.Strip, StripMark{e.TS.Unix(), "revoke"})
			eps = append(eps, &Episode{Start: e.TS, End: e.TS, Kind: "revoke", Names: e.Names, Who: who})
		case (e.Action == audit.Set || e.Action == audit.Rm || e.Action == audit.Import) && e.OK && len(e.Names) > 0:
			eps = append(eps, &Episode{Start: e.TS, End: e.TS, Kind: "change", Names: e.Names, Who: who, Detail: string(e.Action)})
		}
	}
	if in.Locked != nil && in.Locked.At.After(since) {
		eps = append(eps, &Episode{Start: in.Locked.At, End: in.Locked.At, Kind: "lock", Names: in.Locked.Names})
	}
	sort.SliceStable(eps, func(i, j int) bool { return eps[i].Start.After(eps[j].Start) })
	if len(eps) > 40 {
		eps = eps[:40]
	}
	for _, ep := range eps {
		reg.Episodes = append(reg.Episodes, *ep)
	}
	if n := len(reg.Strip); n > 1500 {
		thin := make([]StripMark, 0, 1500)
		for i, m := range reg.Strip {
			if m.K != "run" || i%((n/1500)+1) == 0 {
				thin = append(thin, m)
			}
		}
		reg.Strip = thin
	}
	return reg, today
}

// secretish matches names that read as a secret, which the public tier
// would print unredacted.
var secretish = regexp.MustCompile(`(^|_)(PASSWORD|PASSWD|PWD|PASS|SECRET|TOKEN|PASSPHRASE|KEYPHRASE|PRIVATE|CREDENTIALS?)(_|$)|_KEY$|^KEY$`)

func vaultView(in Input) VaultView {
	cfg := in.Config
	now := in.Now
	window := 30 * 24 * time.Hour
	from := now.Add(-window)
	type use struct {
		runs      int
		last      *time.Time
		spark     []int
		approvals int
		days      map[string]bool
	}
	uses := map[string]*use{}
	get := func(n string) *use {
		u := uses[n]
		if u == nil {
			u = &use{spark: make([]int, 15), days: map[string]bool{}}
			uses[n] = u
		}
		return u
	}
	for _, e := range in.Log {
		switch {
		case e.Action == audit.Exec && e.OK:
			for _, n := range e.Names {
				u := get(n)
				t := e.TS
				u.last = &t
				if e.TS.After(from) {
					u.runs++
					b := int(float64(e.TS.Sub(from)) / float64(window) * 15)
					u.spark[min(max(b, 0), 14)]++
				}
			}
		case granting(e) && e.TS.After(from):
			for _, n := range e.Names {
				u := get(n)
				u.approvals++
				u.days[e.TS.In(now.Location()).Format(time.DateOnly)] = true
			}
		}
	}
	var vaultNames []string
	for _, s := range in.Secrets {
		vaultNames = append(vaultNames, s.Name)
	}
	inBundle := map[string]bool{}
	for _, b := range cfg.Bundle {
		for _, m := range b.Members(vaultNames) {
			inBundle[m] = true
		}
	}
	live := map[string]time.Time{}
	for _, g := range in.Grants {
		if !g.Changed {
			live[g.Name] = g.Expires
		}
	}
	v := VaultView{Total: len(in.Secrets)}
	for _, sec := range in.Secrets {
		u := get(sec.Name)
		row := VaultRow{Name: sec.Name, Tier: string(sec.Tier), Description: sec.Description, Runs: u.runs, LastUsed: u.last, Spark: u.spark}
		if sec.Tier == vault.Public {
			v.Public++
		} else {
			v.Private++
		}
		if exp, ok := live[sec.Name]; ok {
			e := exp
			row.Live = &e
		}
		stale := cfg.Vault.StaleAfter.D()
		// Uses within an hour of storing it were the import checking it
		// worked, not an agent needing it.
		var lastReal *time.Time
		if u.last != nil && u.last.Sub(sec.Created) > time.Hour {
			lastReal = u.last
		}
		switch {
		case lastReal == nil && now.Sub(sec.Created) > stale:
			row.Flags = append(row.Flags, Flag{"warn", fmt.Sprintf("never used in %d days since it was stored", days(now.Sub(sec.Created)))})
			v.Stale++
		case lastReal != nil && now.Sub(*lastReal) > stale:
			row.Flags = append(row.Flags, Flag{"warn", fmt.Sprintf("unused for %d days", days(now.Sub(*lastReal)))})
			v.Stale++
		}
		if strings.TrimSpace(sec.Description) == "" {
			row.Flags = append(row.Flags, Flag{"warn", "no description: agents pick secrets by description"})
		}
		if sec.Tier == vault.Public && secretish.MatchString(sec.Name) {
			row.Flags = append(row.Flags, Flag{"bad", "public, but named like a secret: never redacted from output"})
		}
		if !sec.Updated.IsZero() && now.Sub(sec.Updated) > cfg.Vault.RotateAfter.D() {
			row.Flags = append(row.Flags, Flag{"info", fmt.Sprintf("value is %d days old: rotate?", days(now.Sub(sec.Updated)))})
		}
		// Asked for again and again on the same days: a bundle, or a
		// longer grant, would save touches.
		if sec.Tier == vault.Private && !inBundle[sec.Name] && u.approvals >= 15 && float64(u.approvals) >= 2.5*float64(len(u.days)) {
			row.Flags = append(row.Flags, Flag{"info", fmt.Sprintf("approved %d times on %d days: bundle it, or grant it longer?", u.approvals, len(u.days))})
		}
		if len(row.Flags) > 0 {
			v.Flagged++
		}
		v.Rows = append(v.Rows, row)
	}
	sort.SliceStable(v.Rows, func(i, j int) bool {
		a, b := v.Rows[i], v.Rows[j]
		if a.Runs != b.Runs {
			return a.Runs > b.Runs
		}
		return a.Name < b.Name
	})
	return v
}

func days(d time.Duration) int { return int(d / (24 * time.Hour)) }

func lamp(in Input, s Snapshot) Lamp {
	cfg := in.Config
	now := in.Now
	var l Lamp
	var live []GrantRow
	for _, g := range s.Grants {
		if !g.Changed {
			live = append(live, g)
			l.Count += len(g.Names)
		}
	}
	var soon *GrantRow
	for i := range live {
		if soon == nil || live[i].Expires.Before(soon.Expires) {
			soon = &live[i]
		}
	}
	if soon != nil {
		left := soon.Expires.Sub(now)
		l.Frac = max(0.02, min(1, left.Seconds()/float64(soon.Total)))
	}
	liveLine := ""
	if soon != nil {
		liveLine = fmt.Sprintf("%s live in %s · %s ends in %s",
			plural(l.Count, "secret"), plural(len(live), "grant"), soon.Names[0], Human(soon.Expires.Sub(now)))
	}
	switch {
	case len(s.Asks) > 0:
		a := s.Asks[0]
		l.Level = "asking"
		l.Tooltip = fmt.Sprintf("%s is asking for %s · %s left", a.Who, strings.Join(a.Names, ", "), clockLeft(a.Deadline.Sub(now)))
		if len(s.Asks) > 1 {
			l.Tooltip += fmt.Sprintf(" · %d more waiting", len(s.Asks)-1)
		}
	case len(s.Missed) > 0:
		l.Level = "missed"
		l.Tooltip = fmt.Sprintf("%s missed · click to grant or dismiss", plural(len(s.Missed), "ask"))
	case len(live) == 0 && in.Locked != nil:
		l.Level = "locked"
		l.Tooltip = fmt.Sprintf("Screen locked at %s · revoked %s", in.Locked.At.In(now.Location()).Format("15:04"), plural(len(in.Locked.Names), "secret"))
	case len(live) == 0:
		l.Level = "quiet"
		l.Tooltip = "Nothing live. Agents will ask."
	case ending(cfg, now, soon):
		l.Level = "ending"
	case night(cfg, now):
		l.Level = "night"
	default:
		l.Level = "live"
	}
	if liveLine != "" {
		switch l.Level {
		case "live":
			l.Tooltip = liveLine
		case "ending":
			l.Tooltip = liveLine
			if soon.LastUsed != nil {
				l.Tooltip += " · used " + Human(now.Sub(*soon.LastUsed)) + " ago"
			}
		case "night":
			l.Tooltip = "Still live after " + cfg.Lock.NightAfter + " · " + liveLine
		default:
			l.Tooltip += "\n" + liveLine
		}
	}
	return l
}

func ending(cfg *Config, now time.Time, g *GrantRow) bool {
	if g == nil || g.Expires.Sub(now) > cfg.Expiry.WarnBefore.D() {
		return false
	}
	if cfg.Expiry.OnlyIfInUse != nil && *cfg.Expiry.OnlyIfInUse {
		return g.LastUsed != nil && now.Sub(*g.LastUsed) < 10*time.Minute
	}
	return true
}

func night(cfg *Config, now time.Time) bool {
	if cfg.Lock.NightAfter == "" {
		return false
	}
	from, err1 := clock(cfg.Lock.NightAfter)
	until, err2 := clock(cfg.Lock.NightUntil)
	if err1 != nil || err2 != nil {
		return false
	}
	m := now.Hour()*60 + now.Minute()
	if from <= until {
		return m >= from && m < until
	}
	return m >= from || m < until
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// Human formats d as people say it: 42m, 6h10m, 2d.
func Human(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", max(0, int(d.Seconds())))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		h, m := int(d.Hours()), int(d.Minutes())%60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%02dm", h, m)
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func clockLeft(d time.Duration) string {
	s := max(0, int(d.Seconds()))
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

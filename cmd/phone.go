package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"rsc.io/qr"

	"github.com/NathanielInman/credential-broker/internal/agenterr"
	"github.com/NathanielInman/credential-broker/internal/audit"
	"github.com/NathanielInman/credential-broker/internal/config"
	"github.com/NathanielInman/credential-broker/internal/paths"
	"github.com/NathanielInman/credential-broker/internal/phone"
	"github.com/NathanielInman/credential-broker/internal/vault"
	"github.com/NathanielInman/credential-broker/internal/webpush"
)

// enrollTimeout is how long enroll waits for the phone.
const enrollTimeout = 10 * time.Minute

// pushTimeout bounds one notification to the push service.
const pushTimeout = 10 * time.Second

var phoneNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$`)

// hardenSelf is harden for the phone commands, which run in-process in
// tests, where a non-dumpable test binary would hide its /proc from the
// approvers later tests start.
var hardenSelf = harden

func newPhoneCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "phone",
		Short: "Approve requests on a phone, with a security key plugged into it",
		Long: "A phone enrolled here shows requests on broker's phone page (an HTTPS\n" +
			"page this machine serves on the local network while a request is open)\n" +
			"and gets a notification for each. Approving takes a touch of a security\n" +
			"key plugged into the phone, whose WebAuthn PRF output unlocks the vault.",
		GroupID: groupHuman,
		Args:    usageArgs(cobra.NoArgs),
		RunE:    func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	c.AddCommand(newPhoneEnrollCmd(), newPhoneLsCmd(), newPhoneRmCmd())
	return c
}

func newPhoneEnrollCmd() *cobra.Command {
	var name string
	var page config.PhonePage
	c := &cobra.Command{
		Use:   "enroll [--name NAME] [--origin URL --cert FILE --key FILE] [--listen ADDR]",
		Short: "Enroll a phone and the security key plugged into it",
		Long: "enroll serves the phone page and prints a link (and a QR code) for the\n" +
			"phone to open. There the key makes a WebAuthn credential and gives its PRF\n" +
			"output, and the phone allows notifications. Then the vault is wrapped to\n" +
			"the key as well, which takes one more unlock here (a touch).\n\n" +
			"The first enrollment needs --origin (https://HOST[:PORT], a name the phone\n" +
			"resolves to this machine) and --cert/--key for it; they are kept for the\n" +
			"next. --listen defaults to the origin's port on every interface.",
		Args: usageArgs(cobra.NoArgs),
		RunE: func(c *cobra.Command, _ []string) (err error) {
			defer func() { record(c, audit.Entry{Action: audit.PhoneEnroll}, err) }()
			if err := requireTTY(c.InOrStdin(), "broker phone enroll"); err != nil {
				return err
			}
			if !phoneNameRE.MatchString(name) {
				return agenterr.New(agenterr.Usage, "--name %q: use letters, digits, '.', '_', '-' (up to 32)", name)
			}
			v, err := openVault()
			if err != nil {
				return err
			}
			cfg := v.Config()
			for _, u := range cfg.Phones() {
				if u.Phone.Name == name {
					return agenterr.New(agenterr.Usage, "a phone named %s is enrolled; remove it with broker phone rm %s, or pick another --name", name, name)
				}
			}
			settings, err := phoneSettings(c, cfg.Phone, page)
			if err != nil {
				return err
			}
			// This process will hold the unwrapped private tier identity.
			if err := hardenSelf(); err != nil {
				return err
			}
			return enrollPhone(c, v, name, settings)
		},
	}
	c.Flags().StringVar(&name, "name", "phone", "what to call this phone")
	c.Flags().StringVar(&page.Origin, "origin", "", "the phone page's URL, https://HOST[:PORT]")
	c.Flags().StringVar(&page.Cert, "cert", "", "TLS certificate chain for the origin's host (PEM)")
	c.Flags().StringVar(&page.Key, "key", "", "TLS private key for the certificate (PEM)")
	c.Flags().StringVar(&page.Listen, "listen", "", "address to listen on (default :PORT of the origin)")
	return c
}

// phoneSettings merges the flags given into the page settings in config.
func phoneSettings(c *cobra.Command, saved *config.PhonePage, flags config.PhonePage) (config.PhonePage, error) {
	var page config.PhonePage
	if saved != nil {
		page = *saved
	}
	set := func(flag string, dst *string, val string) {
		if c.Flags().Changed(flag) {
			*dst = val
		}
	}
	set("origin", &page.Origin, flags.Origin)
	set("listen", &page.Listen, flags.Listen)
	set("cert", &page.Cert, flags.Cert)
	set("key", &page.Key, flags.Key)
	if page.Origin == "" || page.Cert == "" || page.Key == "" {
		return page, agenterr.New(agenterr.Usage, "the first enrollment needs --origin, --cert, and --key")
	}
	if _, err := phone.Origin(page); err != nil {
		return page, agenterr.New(agenterr.Usage, "%s", err)
	}
	for _, f := range []*string{&page.Cert, &page.Key} {
		abs, err := filepath.Abs(*f)
		if err != nil {
			return page, err
		}
		*f = abs
	}
	if page.Listen == "" || c.Flags().Changed("origin") && !c.Flags().Changed("listen") {
		u, _ := url.Parse(page.Origin)
		port := u.Port()
		if port == "" {
			port = "443"
		}
		page.Listen = ":" + port
	}
	return page, nil
}

func enrollPhone(c *cobra.Command, v *vault.Vault, name string, page config.PhonePage) (err error) {
	out, errOut := c.OutOrStdout(), c.ErrOrStderr()
	key, err := vapidKey(v.Paths())
	if err != nil {
		return err
	}
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	srv, err := phone.StartEnroll(page, salt, key.Public())
	if err != nil {
		return err
	}
	defer srv.Close()

	fmt.Fprintf(out, "On the phone (on this network), open:\n\n  %s\n\n", srv.EnrollURL())
	if f, ok := out.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		printQR(out, srv.EnrollURL())
	}
	fmt.Fprintln(out, "Plug the security key into the phone and follow the page. Waiting (Ctrl-C to stop)...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var en phone.Enrollment
	select {
	case en = <-srv.Enrolled():
	case <-time.After(enrollTimeout):
		return errors.New("the phone did not enroll in time")
	case <-ctx.Done():
		return errors.New("stopped before the phone enrolled")
	}
	// The page waits to hear how the rest goes here; Close (deferred
	// earlier, so it runs after this) gives it time to fetch the answer.
	defer func() { srv.Finish(err) }()

	// With one YubiKey, the key the phone used is the one that unlocks the
	// vault and has to come back first; with two, the phone's key stays
	// put. Naming the vault's key by serial settles which.
	fmt.Fprintln(out, "\nGot the phone's key. Now unlock the vault on this machine to add it.")
	fmt.Fprintln(out, unlockHint(v.Config()))
	fmt.Fprintln(out, "Then press Enter.")
	lines := readLines(c.InOrStdin())
	ui := vault.TerminalUI(errOut)
	var priv age.Identity
	for attempt := 1; ; attempt++ {
		select {
		case <-lines:
		case <-ctx.Done():
			return errors.New("stopped before the vault was unlocked; the phone was not added")
		}
		fmt.Fprintln(out, "Unlocking: touch the YubiKey in this machine when it blinks, or type the passphrase if asked.")
		if priv, err = v.UnlockPrivate(ui); err == nil {
			break
		}
		if attempt == unlockAttempts {
			return err
		}
		fmt.Fprintf(errOut, "could not unlock: %v\n%s\nPress Enter to try again (Ctrl-C to stop).\n", err, unlockHint(v.Config()))
	}
	cfg := v.Config()
	unlocks := append(slices.Clone(cfg.Unlock), config.Unlock{
		Recipient: en.Identity.Recipient().String(),
		Phone: &config.PhoneKey{
			Name:       name,
			Credential: en.Credential,
			Salt:       base64.RawURLEncoding.EncodeToString(salt),
			Token:      en.Token,
			Push:       en.Push,
		},
	})
	cfg.Phone = &page
	if err := v.SetUnlocks(unlocks, priv, ui); err != nil {
		return err
	}
	// The vault must now open with the phone's key alone.
	if _, err := v.UnlockWith(en.Identity); err != nil {
		return fmt.Errorf("the vault does not open with the phone's key: %w", err)
	}
	fmt.Fprintf(out, "enrolled phone %s\n", name)
	if en.Push == nil {
		fmt.Fprintln(out, "notifications: off (the phone did not allow them); open the page yourself when an agent asks")
	} else if err := pushTo(context.Background(), key, page, *en.Push, phoneMessage("Phone enrolled", "Requests will show up here.", time.Minute)); err != nil {
		fmt.Fprintf(errOut, "broker: test notification failed: %v\n", err)
	} else {
		fmt.Fprintln(out, "sent a test notification")
	}
	printUnlocks(out, v)
	return nil
}

// yubikeySerial finds the serial age-plugin-yubikey wrote in a comment
// above the stub ("#       Serial: 39616611, Slot: 2").
var yubikeySerial = regexp.MustCompile(`(?m)^#\s*Serial:\s*(\d+)`)

// unlockHint tells the human which key unlocks the vault here, by serial
// when the YubiKey identity file says, since with two keys around "the
// YubiKey" could be either.
func unlockHint(cfg *config.Config) string {
	var serials []string
	for _, u := range cfg.Unlock {
		if u.Plugin() != "yubikey" || u.Path == "" {
			continue
		}
		data, err := os.ReadFile(u.Path)
		if err != nil {
			continue
		}
		if m := yubikeySerial.FindSubmatch(data); m != nil && !slices.Contains(serials, string(m[1])) {
			serials = append(serials, string(m[1]))
		}
	}
	switch len(serials) {
	case 0:
		return "Have the YubiKey (or passphrase) that unlocks this vault ready. If the phone just used that same key, move it to this machine now."
	case 1:
		return "Plug in the YubiKey that unlocks this vault, serial " + serials[0] + " (ykman list shows it). " +
			"If the phone just used that same key, move it to this machine now; a different key can stay in the phone."
	}
	return "Plug in a YubiKey that unlocks this vault (serial " + strings.Join(serials, " or ") + "; ykman list shows it). " +
		"If the phone just used that key, move it to this machine now; a different key can stay in the phone."
}

// readLines delivers each line read from r, and keeps delivering once r
// ends, so a closed stdin never blocks.
func readLines(r io.Reader) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		br := bufio.NewReader(r)
		for {
			if _, err := br.ReadString('\n'); err != nil {
				close(ch)
				return
			}
			ch <- struct{}{}
		}
	}()
	return ch
}

// printQR draws text as a QR code with half blocks, two rows per line,
// dark on light whatever the terminal's colors.
func printQR(w io.Writer, text string) {
	code, err := qr.Encode(text, qr.L)
	if err != nil {
		return
	}
	const quiet = 2
	black := func(x, y int) bool {
		x, y = x-quiet, y-quiet
		return x >= 0 && y >= 0 && x < code.Size && y < code.Size && code.Black(x, y)
	}
	n := code.Size + 2*quiet
	var b strings.Builder
	for y := 0; y < n; y += 2 {
		b.WriteString("  \x1b[30;47m")
		for x := range n {
			switch top, bot := black(x, y), black(x, y+1); {
			case top && bot:
				b.WriteString("█")
			case top:
				b.WriteString("▀")
			case bot:
				b.WriteString("▄")
			default:
				b.WriteString(" ")
			}
		}
		b.WriteString("\x1b[0m\n")
	}
	fmt.Fprintln(w, b.String())
}

func newPhoneLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List enrolled phones",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(c *cobra.Command, _ []string) error {
			v, err := openVault()
			if err != nil {
				return err
			}
			cfg, out := v.Config(), c.OutOrStdout()
			phones := cfg.Phones()
			if len(phones) == 0 {
				fmt.Fprintln(out, "no phones enrolled (broker phone enroll)")
				return nil
			}
			if cfg.Phone != nil {
				fmt.Fprintf(out, "page: %s (listening on %s while a request is open)\n", cfg.Phone.Origin, cfg.Phone.Listen)
			}
			for _, u := range phones {
				push := "notifications on"
				if u.Phone.Push == nil {
					push = "notifications off"
				}
				fmt.Fprintf(out, "  %s (%s)\n", u.Phone.Name, push)
			}
			return nil
		},
	}
}

func newPhoneRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm NAME",
		Short: "Remove a phone: the vault no longer opens with its key (one touch)",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(c *cobra.Command, args []string) (err error) {
			defer func() { record(c, audit.Entry{Action: audit.PhoneRm}, err) }()
			if err := requireTTY(c.InOrStdin(), "broker phone rm "+args[0]); err != nil {
				return err
			}
			v, err := openVault()
			if err != nil {
				return err
			}
			cfg := v.Config()
			keep := slices.DeleteFunc(slices.Clone(cfg.Unlock), func(u config.Unlock) bool {
				return u.Phone != nil && u.Phone.Name == args[0]
			})
			if len(keep) == len(cfg.Unlock) {
				return agenterr.New(agenterr.Usage, "no phone named %s is enrolled", args[0])
			}
			if len(keep) == 0 {
				return agenterr.New(agenterr.Usage, "phone %s is the vault's only unlock; enroll something else first", args[0])
			}
			if err := hardenSelf(); err != nil {
				return err
			}
			ui := vault.TerminalUI(c.ErrOrStderr())
			priv, err := v.UnlockPrivate(ui)
			if err != nil {
				return err
			}
			if err := v.SetUnlocks(keep, priv, ui); err != nil {
				return err
			}
			fmt.Fprintf(c.OutOrStdout(), "removed phone %s\n", args[0])
			printUnlocks(c.OutOrStdout(), v)
			return nil
		},
	}
}

// vapidKey loads the key phone notifications are signed with, making it
// on first use.
func vapidKey(p paths.Paths) (*webpush.Key, error) {
	data, err := os.ReadFile(p.VAPIDKey())
	if err == nil {
		return webpush.ParseKey(data)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	key, err := webpush.GenerateKey()
	if err != nil {
		return nil, err
	}
	text, err := key.MarshalText()
	if err != nil {
		return nil, err
	}
	if err := paths.MkdirAll(p.Config); err != nil {
		return nil, err
	}
	return key, paths.WriteFile(p.VAPIDKey(), append(text, '\n'))
}

// phoneMessage is a notification. It never names secrets: it shows on the
// lock screen, and the page has the details.
func phoneMessage(title, body string, ttl time.Duration) webpush.Message {
	payload, _ := json.Marshal(map[string]string{"title": title, "body": body})
	return webpush.Message{Payload: payload, TTL: ttl, Topic: "request", Urgent: true}
}

func pushTo(ctx context.Context, key *webpush.Key, page config.PhonePage, sub config.PushSubscription, m webpush.Message) error {
	ctx, cancel := context.WithTimeout(ctx, pushTimeout)
	defer cancel()
	return webpush.Send(ctx, http.DefaultClient, webpush.Subscription(sub), m, key, page.Origin)
}

// notifyPhones tells each enrolled phone that a request for n secrets is
// waiting until deadline. Failures go to stderr; the page works without.
func notifyPhones(p paths.Paths, cfg *config.Config, n int, deadline time.Time, stderr io.Writer) {
	key, err := vapidKey(p)
	if err != nil {
		fmt.Fprintf(stderr, "broker: phone notification: %v\n", err)
		return
	}
	body := "An agent is asking for a secret."
	if n > 1 {
		body = fmt.Sprintf("An agent is asking for %d secrets.", n)
	}
	m := phoneMessage("broker", body, max(time.Until(deadline), time.Second))
	for _, u := range cfg.Phones() {
		if u.Phone.Push == nil {
			continue
		}
		err := pushTo(context.Background(), key, *cfg.Phone, *u.Phone.Push, m)
		switch {
		case errors.Is(err, webpush.ErrGone):
			fmt.Fprintf(stderr, "broker: phone %s no longer takes notifications; ask the human to run broker phone rm %s and enroll it again\n", u.Phone.Name, u.Phone.Name)
		case err != nil:
			fmt.Fprintf(stderr, "broker: notifying phone %s: %v\n", u.Phone.Name, err)
		}
	}
}

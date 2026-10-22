package keyring

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/NathanielInman/credential-broker/internal/keyring/keyringtest"
)

// envHelper makes the test binary act as another process: "read NAME",
// "readid SERIAL", or "add NAME VALUE". It prints its session keyring ID,
// then a newline, then any value read.
const envHelper = "BROKER_KEYRING_HELPER"

func TestMain(m *testing.M) {
	if cmd := os.Getenv(envHelper); cmd != "" {
		os.Exit(runHelper(strings.Fields(cmd)))
	}
	os.Exit(keyringtest.Main(m))
}

func runHelper(args []string) int {
	session, err := keyringtest.SessionKeyring()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(session)
	k, err := User()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var value []byte
	switch args[0] {
	case "read":
		value, err = k.Read(args[1])
	case "readid":
		var id int
		if id, err = strconv.Atoi(args[1]); err == nil {
			value, err = readKey(id)
		}
	case "add":
		_, err = k.Add(args[1], []byte(args[2]), time.Minute)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	os.Stdout.Write(value)
	return 0
}

// helper runs the test binary as a helper process in a new session
// keyring and returns its session keyring ID and value.
func helper(t *testing.T, args ...string) (session int, value string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), envHelper+"="+strings.Join(args, " "))
	var out []byte
	err = keyringtest.InNewSession(func() (err error) {
		out, err = cmd.Output()
		return err
	})
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("helper %v: %v: %s", args, err, exit.Stderr)
		}
		t.Fatalf("helper %v: %v", args, err)
	}
	first, rest, _ := strings.Cut(string(out), "\n")
	if session, err = strconv.Atoi(first); err != nil {
		t.Fatalf("helper output %q", out)
	}
	return session, rest
}

func testKeyring(t *testing.T) *Keyring {
	t.Helper()
	keyringtest.Require(t)
	k, err := User()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		gs, _ := k.List()
		for _, g := range gs {
			_, _ = k.Revoke(g.Name)
		}
	})
	return k
}

func TestAddReadRevoke(t *testing.T) {
	k := testKeyring(t)
	if _, err := k.Read("NOPE"); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("Read ungranted: %v", err)
	}

	before := time.Now()
	expires, err := k.Add("API_TOKEN", []byte("s3cret-value"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if d := expires.Sub(before); d < time.Hour-time.Second || d > time.Hour+time.Second {
		t.Fatalf("expires in %s, want 1h", d)
	}
	if v, err := k.Read("API_TOKEN"); err != nil || string(v) != "s3cret-value" {
		t.Fatalf("Read = %q, %v", v, err)
	}
	g, err := k.Lookup("API_TOKEN")
	if err != nil || !g.Expires.Equal(expires) {
		t.Fatalf("Lookup = %+v, %v; want expiry %s", g, err, expires)
	}

	// A second grant replaces the value and expiry in place.
	if _, err := k.Add("API_TOKEN", []byte("rotated"), 2*time.Hour); err != nil {
		t.Fatal(err)
	}
	if v, err := k.Read("API_TOKEN"); err != nil || string(v) != "rotated" {
		t.Fatalf("Read after regrant = %q, %v", v, err)
	}
	gs, err := k.List()
	if err != nil || len(gs) != 1 || gs[0].Name != "API_TOKEN" || time.Until(gs[0].Expires) < time.Hour {
		t.Fatalf("List = %+v, %v", gs, err)
	}

	if live, err := k.Revoke("API_TOKEN"); err != nil || !live {
		t.Fatalf("Revoke = %v, %v", live, err)
	}
	if _, err := k.Read("API_TOKEN"); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("Read after revoke: %v", err)
	}
	if _, err := k.Lookup("API_TOKEN"); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("Lookup after revoke: %v", err)
	}
	if live, err := k.Revoke("API_TOKEN"); err != nil || live {
		t.Fatalf("second Revoke = %v, %v", live, err)
	}
}

func TestLargeValue(t *testing.T) {
	k := testKeyring(t)
	// Over a user key's 32 KiB cap and the default 20000 byte quota.
	value := []byte(strings.Repeat("x", 128<<10))
	if _, err := k.Add("BIG", value, time.Minute); err != nil {
		t.Fatal(err)
	}
	if v, err := k.Read("BIG"); err != nil || string(v) != string(value) {
		t.Fatalf("Read = %d bytes, %v", len(v), err)
	}
}

func TestExpiry(t *testing.T) {
	k := testKeyring(t)
	if _, err := k.Add("SHORT", []byte("short-lived"), time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Add("LONG", []byte("long-lived"), time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Read("SHORT"); err != nil {
		t.Fatalf("Read before expiry: %v", err)
	}
	// The kernel expires at a whole second, so allow two.
	time.Sleep(2100 * time.Millisecond)

	if _, err := k.Read("SHORT"); !errors.Is(err, ErrExpired) {
		t.Fatalf("Read after expiry: %v, want ErrExpired", err)
	}
	if _, err := k.Lookup("SHORT"); !errors.Is(err, ErrExpired) {
		t.Fatalf("Lookup after expiry: %v, want ErrExpired", err)
	}
	gs, err := k.List()
	if err != nil || len(gs) != 1 || gs[0].Name != "LONG" {
		t.Fatalf("List = %+v, %v", gs, err)
	}
	if live, err := k.Revoke("SHORT"); err != nil || live {
		t.Fatalf("Revoke expired = %v, %v", live, err)
	}
	// A new grant displaces the expired keys.
	if _, err := k.Add("SHORT", []byte("again"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if v, err := k.Read("SHORT"); err != nil || string(v) != "again" {
		t.Fatalf("Read after regrant = %q, %v", v, err)
	}
}

func TestBadDuration(t *testing.T) {
	k := testKeyring(t)
	if _, err := k.Add("X", []byte("value"), 0); err == nil {
		t.Fatal("zero duration granted")
	}
}

func TestPermissions(t *testing.T) {
	k := testKeyring(t)
	if _, err := k.Add("PERM", []byte("value"), time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, typDesc := range [][2]string{{secretType, secretPrefix + "PERM"}, {expiryType, expiryPrefix + "PERM"}} {
		id, err := unix.KeyctlSearch(k.ring, typDesc[0], typDesc[1], 0)
		if err != nil {
			t.Fatal(err)
		}
		d, err := unix.KeyctlString(unix.KEYCTL_DESCRIBE, id)
		if err != nil {
			t.Fatal(err)
		}
		f := strings.Split(d, ";")
		perm, err := strconv.ParseUint(f[3], 16, 32)
		if err != nil || perm != permission {
			t.Errorf("%s perm %s, want %08x", typDesc[1], f[3], permission)
		}
		if f[1] != strconv.Itoa(os.Getuid()) {
			t.Errorf("%s uid %s", typDesc[1], f[1])
		}
	}
}

// TestCrossProcess is the property agents depend on: a grant made in one
// process (the human's terminal) is readable from another process of the
// same uid with a different session keyring (the agent), and the other way
// round.
func TestCrossProcess(t *testing.T) {
	k := testKeyring(t)
	mySession, err := keyringtest.SessionKeyring()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k.Add("FROM_PARENT", []byte("parent-value"), time.Minute); err != nil {
		t.Fatal(err)
	}

	session, value := helper(t, "read", "FROM_PARENT")
	if session == mySession {
		t.Fatalf("helper shares session keyring %d", session)
	}
	if value != "parent-value" {
		t.Fatalf("helper read %q", value)
	}

	// By serial: the helper's new session keyring does not link the user
	// keyring, so it does not possess the key and relies on the user
	// read permission.
	id, err := unix.KeyctlSearch(k.ring, secretType, secretPrefix+"FROM_PARENT", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, value := helper(t, "readid", strconv.Itoa(id)); value != "parent-value" {
		t.Fatalf("helper read by serial %q", value)
	}

	helper(t, "add", "FROM_CHILD", "child-value")
	if v, err := k.Read("FROM_CHILD"); err != nil || string(v) != "child-value" {
		t.Fatalf("Read child grant = %q, %v", v, err)
	}
}

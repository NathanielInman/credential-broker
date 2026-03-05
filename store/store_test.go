package store

import (
	"os"
	"testing"
	"time"
)

func tempDB(t *testing.T) *Store {
	t.Helper()
	f, err := os.CreateTemp("", "broker-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	path := f.Name()
	t.Cleanup(func() { os.Remove(path) })

	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSetGet(t *testing.T) {
	s := tempDB(t)

	err := s.Set("users", "alice", []byte(`{"name":"alice"}`), 0)
	if err != nil {
		t.Fatal(err)
	}

	val, err := s.Get("users", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != `{"name":"alice"}` {
		t.Errorf("got %s, want %s", val, `{"name":"alice"}`)
	}
}

func TestGetMissing(t *testing.T) {
	s := tempDB(t)

	val, err := s.Get("users", "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Errorf("expected nil for missing key, got %s", val)
	}
}

func TestDelete(t *testing.T) {
	s := tempDB(t)

	s.Set("users", "bob", []byte("data"), 0)
	s.Delete("users", "bob")

	val, _ := s.Get("users", "bob")
	if val != nil {
		t.Error("expected nil after delete")
	}
}

func TestTTLExpiry(t *testing.T) {
	s := tempDB(t)

	// Store uses second-precision timestamps, so TTL must be >= 1s
	s.Set("sessions", "sess1", []byte(`"active"`), 1*time.Second)

	// Should exist immediately
	val, _ := s.Get("sessions", "sess1")
	if val == nil {
		t.Fatal("expected value before expiry")
	}

	// Wait for expiry
	time.Sleep(2 * time.Second)

	val, _ = s.Get("sessions", "sess1")
	if val != nil {
		t.Errorf("expected nil after TTL expiry, got %s", val)
	}
}

func TestTTLNoExpiry(t *testing.T) {
	s := tempDB(t)

	s.Set("config", "setting", []byte(`"value"`), 0)

	time.Sleep(10 * time.Millisecond)

	val, _ := s.Get("config", "setting")
	if val == nil {
		t.Error("TTL=0 entry should not expire")
	}
}

func TestKeys(t *testing.T) {
	s := tempDB(t)

	s.Set("scopes", "app1", []byte("{}"), 0)
	s.Set("scopes", "app2", []byte("{}"), 0)

	keys, err := s.Keys("scopes")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestGetAll(t *testing.T) {
	s := tempDB(t)

	s.Set("scopes", "a", []byte(`"v1"`), 0)
	s.Set("scopes", "b", []byte(`"v2"`), 0)

	all, err := s.GetAll("scopes")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}
}

func TestCleanup(t *testing.T) {
	s := tempDB(t)

	s.Set("sessions", "expired", []byte(`"data"`), 1*time.Second)
	s.Set("sessions", "active", []byte(`"data"`), 1*time.Hour)

	// Wait long enough that the 1s TTL is definitely expired
	time.Sleep(3 * time.Second)
	s.Cleanup()

	keys, _ := s.Keys("sessions")
	if len(keys) != 1 {
		// Debug: check what keys remain
		for _, k := range keys {
			t.Logf("remaining key: %s", k)
		}
		t.Errorf("expected 1 key after cleanup, got %d", len(keys))
	}
}

// Package paths resolves where broker keeps its config, vault, and log.
//
// BROKER_HOME, when set, is a single root with config/, data/, and state/
// beneath it. Otherwise the XDG base directories are used, each with a
// broker/ subdirectory.
package paths

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	DirMode  os.FileMode = 0o700
	FileMode os.FileMode = 0o600
)

// Paths holds the three broker directories.
type Paths struct {
	Config string
	Data   string
	State  string
}

// Resolve reads BROKER_HOME and the XDG variables from the environment.
func Resolve() (Paths, error) {
	if root := os.Getenv("BROKER_HOME"); root != "" {
		if !filepath.IsAbs(root) {
			return Paths{}, fmt.Errorf("BROKER_HOME must be an absolute path, got %q", root)
		}
		return Paths{
			Config: filepath.Join(root, "config"),
			Data:   filepath.Join(root, "data"),
			State:  filepath.Join(root, "state"),
		}, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		Config: filepath.Join(xdg("XDG_CONFIG_HOME", home, ".config"), "broker"),
		Data:   filepath.Join(xdg("XDG_DATA_HOME", home, ".local", "share"), "broker"),
		State:  filepath.Join(xdg("XDG_STATE_HOME", home, ".local", "state"), "broker"),
	}, nil
}

// xdg returns the named variable, or home joined with fallback when it is
// unset or relative (the XDG spec says relative values are invalid).
func xdg(name, home string, fallback ...string) string {
	if v := os.Getenv(name); filepath.IsAbs(v) {
		return v
	}
	return filepath.Join(append([]string{home}, fallback...)...)
}

func (p Paths) ConfigFile() string      { return filepath.Join(p.Config, "config.json") }
func (p Paths) PublicKey() string       { return filepath.Join(p.Data, "public.key") }
func (p Paths) PrivateKey() string      { return filepath.Join(p.Data, "private.key.age") }
func (p Paths) SecretsDir() string      { return filepath.Join(p.Data, "secrets") }
func (p Paths) LogFile() string         { return filepath.Join(p.State, "log.jsonl") }
func (p Paths) TierDir(t string) string { return filepath.Join(p.SecretsDir(), t) }

// Ensure creates the config, data, and state directories at DirMode.
func (p Paths) Ensure() error {
	for _, d := range []string{p.Config, p.Data, p.State} {
		if err := MkdirAll(d); err != nil {
			return err
		}
	}
	return nil
}

// MkdirAll creates dir and any missing parents at DirMode, then tightens dir
// itself to DirMode in case it already existed with looser permissions.
func MkdirAll(dir string) error {
	if err := os.MkdirAll(dir, DirMode); err != nil {
		return err
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if fi.Mode().Perm() != DirMode {
		return os.Chmod(dir, DirMode)
	}
	return nil
}

// WriteFile atomically replaces name with data at FileMode: it writes a
// temporary file in the same directory, syncs it, and renames it over name,
// so readers never see a partial file.
func WriteFile(name string, data []byte) error {
	return WriteFileFunc(name, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// WriteFileFunc is WriteFile for content too large to hold in memory: write
// streams it into the temporary file. If write fails, name is untouched.
func WriteFileFunc(name string, write func(io.Writer) error) error {
	f, err := os.CreateTemp(filepath.Dir(name), "."+filepath.Base(name)+".tmp*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // no-op once renamed

	if err := f.Chmod(FileMode); err != nil {
		f.Close()
		return err
	}
	if err := write(f); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, name)
}

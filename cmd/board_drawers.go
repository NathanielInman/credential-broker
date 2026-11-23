package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// One drawer open at a time across the waybar apps, the protocol of
// bar-anchor's drawers module (github.com/NathanielInman/bar-anchor): each
// drawer listens on <name>.sock in $XDG_RUNTIME_DIR/bar-anchor/drawers, and
// a drawer that opens writes "hide\n" to every other socket there. There is
// no list of apps; a socket nobody listens on is removed.

const drawerName = "broker-board"

var drawerTimeout = 300 * time.Millisecond

func drawersDir() string {
	rt := os.Getenv("XDG_RUNTIME_DIR")
	if rt == "" {
		return ""
	}
	return filepath.Join(rt, "bar-anchor", "drawers")
}

// listenDrawers calls onHide each time another app's drawer opens, until
// the returned func stops it.
func listenDrawers(dir, name string, onHide func()) (func(), error) {
	if dir == "" {
		return func() {}, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name+".sock")
	if c, err := net.DialTimeout("unix", path, drawerTimeout); err == nil {
		c.Close()
		return nil, fmt.Errorf("another %s drawer is listening on %s", name, path)
	}
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetReadDeadline(time.Now().Add(drawerTimeout))
				line, _ := bufio.NewReader(conn).ReadString('\n')
				if strings.TrimSpace(line) == "hide" {
					onHide()
				}
			}()
		}
	}()
	return func() { _ = ln.Close() }, nil
}

// drawerOpened tells every other drawer in dir to close and returns how
// many took it.
func drawerOpened(dir, name string) int {
	if dir == "" {
		return 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	told := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sock") || e.Name() == name+".sock" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		conn, err := net.DialTimeout("unix", path, drawerTimeout)
		if err != nil {
			if errors.Is(err, syscall.ECONNREFUSED) {
				_ = os.Remove(path)
			}
			continue
		}
		_ = conn.SetWriteDeadline(time.Now().Add(drawerTimeout))
		if _, err := conn.Write([]byte("hide\n")); err == nil {
			told++
		}
		conn.Close()
	}
	return told
}

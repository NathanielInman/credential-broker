# credential-broker

Notes for agents working in this repo.

## Platforms

The suite targets Linux, Windows and macOS; `platforms` in
`artwork/hero/suite.toml` lists where this app builds today, and CI and the
pre-push hook hold it to that list.

- Turn the hook on once per clone: `git config core.hooksPath .githooks`.
  Before a push it runs `go vet` with `GOOS` set for every platform listed.
  A failure there is a broken build on that platform: fix it, don't push
  past it with `--no-verify`.
- Code for one platform goes in a file with a build constraint
  (`_linux.go`, `//go:build windows`).
- A test that needs something one platform has (broker-board, Unix socket
  behavior, a POSIX shell, a path layout) skips elsewhere with
  `runtime.GOOS`.

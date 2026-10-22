# credential-broker v2 design

A local, single-user secret store built for AI agents. No server. Humans
authorize with a hardware touch (YubiKey); agents use secrets without ever
seeing them. The threat model is accidental leakage (tokens in transcripts,
logs, committed files, ambient shell env), not a hostile agent holding an
active grant.

## Principles

- Nothing is ambient. Secrets live in no shell rc file and no global env.
- Agents run commands with secrets; they do not read secrets.
- Intent comes from a human touching a hardware key. Agents never do auth
  ceremony; they get a structured error telling them what to ask the human.
- Every read is logged with which process tree asked.
- Storage is plain `age` files, so the vault stays recoverable with the
  stock `age` CLI even if this tool disappears.

## Binary and platform

- Binary name `broker`, Go, module `github.com/NathanielInman/credential-broker`.
- Linux only for v2 (grants use the kernel keyring). Other GOOS builds compile
  but every command returns "unsupported platform".
- Libraries: `filippo.io/age` (including `filippo.io/age/plugin` client
  support for hardware identities), `golang.org/x/sys/unix` (keyctl),
  `golang.org/x/term`, `github.com/spf13/cobra`. No cgo.

## Tiers (replace v1 scopes)

Every secret has exactly one tier:

- `public`: low-sensitivity values kept here for convenience (usernames,
  endpoints, low-risk tokens). Readable by `get` and `exec` at any time, no
  touch, not redacted.
- `private` (default): needs a grant to use via `exec`, and a fresh touch to
  print via `get`. Always redacted from `exec` output.

Secret names are environment variable names (`^[A-Z_][A-Z0-9_]*$`). The name
is what `exec` exports. Each secret carries an optional one-line description
so agents can discover what it is for.

## Layout (XDG)

```
$XDG_CONFIG_HOME/broker/config.json     # unlock identity paths/stubs, recipients, defaults
$XDG_DATA_HOME/broker/public.key        # age X25519 identity for the public tier (0600)
$XDG_DATA_HOME/broker/private.key.age   # age X25519 identity for the private tier,
                                        #   encrypted to the unlock recipients (hardware)
$XDG_DATA_HOME/broker/secrets/<tier>/<NAME>.age   # value, encrypted to that tier's recipient
$XDG_DATA_HOME/broker/secrets/<tier>/<NAME>.json  # metadata: description, created, updated
$XDG_STATE_HOME/broker/log.jsonl        # append-only audit log
```

Directories 0700, files 0600. Honor `BROKER_HOME` as a single override root
(config/, data/, state/ beneath it) for tests.

Key consequences:

- Writing a private secret needs only the private tier's *recipient* (public
  key, kept in config), so `set` never needs a touch.
- Reading a private secret needs the private identity, which only a touch can
  unwrap.

## Unlock identities

`broker init --identity <file>` (repeatable). Each file is an age identity
file: a hardware stub from `age-plugin-yubikey --identity` (touch required),
or any other age plugin identity, or a plain `AGE-SECRET-KEY-` file. Plain
keys are allowed only with `--insecure-plain-identity` and exist for tests and
machines without hardware. `init` derives recipients from the identities,
generates the two tier identities, and wraps the private one. Plugin prompts
("touch your YubiKey") go to stderr / `/dev/tty` through the age plugin
ClientUI.

## Grants (kernel keyring)

`broker grant NAME... [--for 2h]` (or `--all`): one touch unwraps the private
identity, decrypts just the named secrets, and stores each value in the
kernel user keyring as `big_key` `broker:secret:<NAME>` with
`KEYCTL_SET_TIMEOUT`. `big_key` rather than `user` because it holds up to
1 MiB and keeps large payloads out of the small per-uid key quota
(`kernel.keys.maxbytes`, 20000 bytes by default). The private identity itself
is never cached. The kernel drops the key at expiry; no daemon. Keys are
created in the thread keyring, given their timeout and permissions, then
linked into the user keyring, so a crash never leaves a timeless secret
behind and a re-grant replaces the old key atomically. Permissions are
possessor all plus user view/read/search, so any process of the same uid can
read the key whether or not it possesses it. The user keyring is per uid,
not per login session, so agents in other session keyrings see the grant
(verified: `TestCrossProcess`, `TestGrantVisibleToOtherSession`, and a manual
check with `keyctl session -` and `setsid` against the real user keyring).

Caveat: user keyrings are per user namespace. A process inside another user
namespace (bubblewrap, flatpak, and agent sandboxes built on them; bwrap
always unshares the user namespace when unprivileged) has its own empty user
keyring and sees no grants. `exec` must run outside such sandboxes.

Default duration comes from config (`default_grant`, initially `1h`); max
`24h` unless `--force`. `broker revoke [NAME...|--all]` unlinks keys. `ls`
shows remaining TTL by reading key timeouts (track expiry in a companion
`broker:expiry:<NAME>` key or the description if the kernel does not expose
it).

## Commands

Human-facing (rare):

- `broker init --identity FILE...`
- `broker set NAME [--public|--private] [--desc TEXT] [--stdin]`: value from
  a hidden TTY prompt, or stdin with `--stdin`. New secrets default to
  private. Overwrites keep the tier and description unless told: `--public`
  or `--private` moves the secret, `--desc` replaces the description.
- `broker rm NAME`
- `broker grant NAME... [--for DUR] | --all`
- `broker revoke [NAME...] | --all`
- `broker get NAME`: prints the value. Public: immediate. Private: always a
  fresh touch, a grant does not satisfy it (grants are for exec only).
- `broker import FILE [--public]`: parse `export NAME=value` / `NAME=value`
  lines (quotes handled) from shell env files like `~/.zshenv`, store them,
  print which lines can now be deleted. Never prints values.
- `broker backup -o FILE` / `broker restore FILE` (see below)
- `broker log [--since DUR] [--name NAME] [--json]`

Agent-facing (frequent, no prompts ever):

- `broker ls [--json]`: name, tier, description, granted (bool), grant
  expiry. Never values.
- `broker exec NAME... -- CMD [ARGS...]`: run CMD with the named secrets added
  to its environment. Exit code, stdin, and signals pass through.

## exec behaviour

- Resolve each NAME: public from disk, private from the keyring. Any private
  NAME without a live grant fails before running anything: exit 77 and
  print to stderr one JSON line:
  `{"error":"no_grant","names":["X"],"ask_human":"broker grant X --for 1h"}`.
  Unknown names: exit 78, `{"error":"unknown_secret","names":[...]}`.
- Redaction: when the child's stdout or stderr is not a TTY (the agent case),
  pipe it through a streaming redactor that replaces every private value with
  `[REDACTED:NAME]`, holding back `maxlen-1` bytes so matches across read
  boundaries are caught. Skip values shorter than 6 bytes. When the stream is
  a TTY (a human), pass it through untouched so interactive tools work.
- Never put values in argv, never write them to disk.

## Audit log

One JSON object per line: `ts`, `action` (`set|rm|grant|revoke|get|exec|import|backup|restore`),
`names`, `tier`, `ok`, `error`, `cwd`, `cmd` (exec's child argv[0] only),
`pid`, and `ancestry`: the process chain from `/proc/<pid>/stat` + `comm`
up to pid 1, e.g. `["broker","bash","claude","zsh","alacritty"]`, so it shows
which agent asked. Never log values. Log failures too (a denied `no_grant`
is the most useful line in the file).

## Backup and restore

- `broker backup -o FILE`: tar of public.key, the *unwrapped* private
  identity (needs a touch), secrets/, config.json, and optionally the log,
  encrypted with an age scrypt passphrase recipient (scrypt must be the only
  recipient in an age file, so this is a separate file from the vault).
  Prompt twice. Offer `--generate-passphrase` (7 diceware words from the EFF
  long list, embedded) and warn when a typed passphrase is under ~20 chars.
- `broker restore FILE [--identity FILE...] [--force]`: passphrase, then
  rewrites the vault and wraps the private identity to the given identities,
  or to the recipients stored in the backup's config (same YubiKey travels
  with you). Refuses to overwrite an existing vault without `--force`.
- The backup file is safe to keep anywhere (git, R2, NAS). Its strength is
  the passphrase; offline guessing has no rate limit.

## Errors and output for agents

- Every failure an agent can hit has a stable JSON line on stderr with an
  `error` code and, where a human must act, an `ask_human` command string.
- `--json` on read commands for machine output; human output otherwise.
- No command an agent runs ever blocks on a prompt: if a prompt would be
  needed and stdin is not a TTY, fail with `{"error":"needs_human",...}`.

## Testing

- `BROKER_HOME` in a temp dir plus a plain identity
  (`--insecure-plain-identity`) covers everything but the touch itself.
- Keyring tests rerun the test binary in a new user namespace mapping only
  the caller's uid (`keyringtest.Main`), started from a fresh session keyring
  (`KEYCTL_JOIN_SESSION_KEYRING`). A session keyring alone does not isolate
  the user keyring, which is per uid; a user namespace has its own, so tests
  never touch the real one and leave nothing behind even if they crash.
  Without user namespaces the keyring tests skip.
- Run Go builds and tests under
  `systemd-run --user --scope -q -p MemoryMax=4G nice -n 19 go test ./...`.

## Out of scope for v2

Server mode, multi-user, scopes/ACLs, macOS/Windows (Keychain / Touch ID
via `age-plugin-se` is a natural later port), credential-injecting proxy,
rotation.

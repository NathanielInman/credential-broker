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
  ceremony: they get a structured error telling them what to ask the human,
  or ask for them through the request window.
- Every read is logged with which process tree asked.
- Storage is plain `age` files, so the vault stays recoverable with the
  stock `age` CLI even if this tool disappears.

## Binary and platform

- Binary name `broker`, Go, module `github.com/NathanielInman/credential-broker`.
- Linux, macOS, and Windows. Other GOOS builds compile but every command
  returns "unsupported platform". The platform seams are `internal/keyring` (kernel
  keyring, or the grant agent), `internal/procinfo` (who asked: `/proc`, or
  sysctl and libproc), and `cmd/platform_*.go` (`harden`, `desktopSession`).
- Libraries: `filippo.io/age` (including `filippo.io/age/plugin` client
  support for hardware identities), `golang.org/x/sys/unix` (keyctl),
  `golang.org/x/term`, `github.com/spf13/cobra`, `rsc.io/qr` (the
  enrollment link's QR code). No cgo on linux. On macOS
  cgo reads a requester's executable and working directory through libproc;
  a build without it reads the executable from `kern.procargs2` and shows no
  directory.
- The request window is a second program, `broker-dialog` (its own module
  in `broker-dialog/`, cgo with GTK 3 and WebKitGTK 4.1 on linux, Cocoa and
  WKWebView on macOS, go-webview2 (pure Go, WebView2) on Windows, behind a
  small `window` interface), so `broker` links nothing and `go test ./...`
  from the root needs no GTK. On Windows it is built `-H=windowsgui` and
  posts through a bound `brokerPost` function instead of a WebKit message
  handler; closing it by its X is told apart from quitting by a flag.

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
$XDG_CONFIG_HOME/broker/vapid.key       # signs phone notifications (Web Push VAPID, 0600)
$XDG_CONFIG_HOME/broker/fcm-service-account.json  # sends the Android app's notifications (FCM, 0600)
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
file of one of three kinds, and each kind is a **zone** (`config.Zone`):

- `hardware`: a plugin stub (`AGE-PLUGIN-...`) from `age-plugin-yubikey
  --identity`, `age-plugin-se keygen` (Touch ID), or any other age plugin.
  The stub is kept in config, so unlocking works without the file. When the
  plugin wrote its recipient in a comment above the stub (`# public key:
  age1se1...`, `#    Recipient: age1yubikey1...`), config keeps that too and
  init wraps to it; age-plugin-se cannot derive a recipient from a stub.
  Otherwise the plugin derives one from the stub.
- `passphrase`: an identity file encrypted with `age -p` (binary or
  armored). `init` asks for the passphrase through the ClientUI to read the
  recipient; config keeps the path, the recipient, and `"passphrase": true`.
  Unlocking reads the file again and asks again. The identity inside must be
  X25519 or hybrid; a plugin stub inside is refused.
- `plain`: an `AGE-SECRET-KEY-` file, allowed only with
  `--insecure-plain-identity`, for tests.

A fourth kind comes only from `broker phone enroll`, never from a file: a
**phone key** (`Unlock.Phone`), in the hardware zone. Its recipient is the
X25519 key a security key's WebAuthn PRF output derives (see
[Phone page](#phone-page)). Local unlocking skips it; only the phone page
can answer for it.

Any unlock opens the private tier, so the vault's zone is its weakest
unlock's. `init` and `restore` print each unlock's zone and the vault's.
Unlocking runs in two rounds: plain and plugin identities first (age tries
native identities before plugins, so a plain key never prompts), then
passphrase files, which ask only if the first round failed, so a vault with
a YubiKey and a spare passphrase file never asks for the passphrase while
the YubiKey works. A wrong passphrase is reported as such, not as "no
identity matched".

A U2F-only key is not a zone: it can't derive a key, so it could only be a
check in broker's code, and the vault files are readable around it. A touch
added to a passphrase is not one either, since the passphrase alone still
decrypts the file. `init` derives recipients from the identities, generates
the two tier identities, and wraps the private one. Plugin prompts ("touch
your YubiKey") and passphrase prompts go to stderr / `/dev/tty` through the
age plugin ClientUI.

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

One user keyring serves every vault of the uid, so a vault under
`BROKER_HOME` names its keys `broker:<namespace>:secret:<NAME>` (and
`...:expiry:...`), the namespace being the first 12 hex digits of the
SHA-256 of its data directory. Its grants, `revoke --all`, and the revoke
after `restore --force` stay inside it, and the default vault keeps the
plain names (`TestNamespaces`). The grant agent needs no namespace: its
socket is in the vault's own runtime directory.

User keyrings are per user namespace, though. A process inside another user
namespace (bubblewrap, flatpak, and agent sandboxes built on them; bwrap
always unshares the user namespace when unprivileged) has its own empty user
keyring and sees no grants there. So every Linux grant also goes to the
grant agent (next section), whose socket a sandbox can reach; see
[Grants in sandboxes (Linux)](#grants-in-sandboxes-linux).

A grant is a copy of the value, so it outlives a change to the secret.
Each grant therefore carries the vault's `Version` of the secret: the first
16 bytes of the SHA-256 of its `.age` file, hex. age encrypts afresh every
time, so any write (`set`, `import --overwrite`, `restore`, an edit with
the age CLI) changes it, and it says nothing about the value. In the kernel
it follows the expiry in the companion key (`<unix seconds> <version>`);
the agent keeps it beside the value. A grant whose version no longer
matches the file is *changed*: not live, so exec refuses it, `ls` reports
`"granted": false, "changed": true`, and `request` asks for it again. It is
left in place until it expires or is replaced, so the next exec can still
say why. `grant` reads the version before decrypting, and the kernel gets
the value before the version that names it while readers look up the
version before reading the value, so every race reads as changed (one
needless request), never as current. A grant without a version (made by
an older broker) cannot be checked and counts as live.

Default duration comes from config (`default_grant`, initially `1h`); max
`24h` unless `--force`. Granting a public secret is a no-op with a note.
`broker revoke NAME... | --all` unlinks keys; `rm` also drops the secret's
grant. The kernel does not expose a key's timeout through keyctl, so each
grant has a companion `user` key `broker:expiry:<NAME>` (Unix seconds, same
timeout) that `ls` reads. An expired key reads as `EKEYEXPIRED` until the
kernel garbage collects it (`kernel.keys.gc_delay`, 5 minutes), then as
missing, so "expired" and "never granted" are only distinguishable for that
window. If the keyring is unavailable (keyctl blocked), `ls` still lists
secrets, shows none as granted, and says why on stderr.

## Grant agent

macOS and Windows have no kernel keyring, so `internal/keyring` there is a
client for `broker grantd`, a per-user agent that holds grants in memory.
Linux runs the same agent beside the kernel keyring, for sandboxes:

- The first `Add` that finds no agent starts one (`keyring.StartAgent`:
  this executable's `grantd`, in a session of its own) and waits up to 5s
  for its socket. Reads, lists, and revokes never start one: no agent
  means no grants.
- One agent per user: it takes an flock on `run/grantd.lock` (waiting 2s,
  since an exiting agent holds it briefly), removes a stale socket, and
  listens on `run/grantd.sock` with umask 077. A runtime directory too deep
  for sun_path (104 bytes) gets `$TMPDIR/broker-<hash>.sock` instead.
- One JSON request and response per connection (`add`, `read`, `lookup`,
  `list`, `revoke`). It answers only its own user, as the kernel keyring
  does (any process of that user can read a grant): by uid
  (`LOCAL_PEERCRED`) on macOS, and on Windows by the token SID of the
  peer's process (`SIO_AF_UNIX_GETPEERPID`). On Windows the socket file
  also inherits the profile directory's ACL, and the lock file is opened
  with delete sharing so a running agent never blocks removing its
  directory.
- Each grant has a timer for its exact TTL; at expiry the value is zeroed
  and dropped. With no grants for 30s the agent exits. Nothing is written
  to disk, so a reboot forgets every grant.
- It calls `harden`. On macOS that is `PT_DENY_ATTACH`, and the hardened
  runtime (`codesign -o runtime`) is what stops other processes of the
  user reading its memory. On Windows it sets a protected process DACL
  granting the user and OWNER RIGHTS only query-limited-information,
  synchronize, and terminate, so no PROCESS_VM_READ; an administrator with
  SeDebugPrivilege still gets in, and handles opened before (the starter's)
  keep their access. Terminate is there so the user can end a stuck agent,
  dropping its grants: Task Manager's End task works, but PowerShell's
  `Stop-Process` asks for more rights than terminate and is refused. Windows
  starts it with CREATE_NEW_PROCESS_GROUP and
  CREATE_NO_WINDOW, so it survives its console and the age plugins it runs
  get no window.
- `ErrExpired` does not exist here: an expired grant reads as not granted.

## Grants in sandboxes (Linux)

`keyring.Keyring` on Linux is the kernel user keyring with the grant agent
beside it (`internal/keyring/store_linux.go`):

- `Add` writes the kernel keyring, then the agent, with the same duration
  (the agent's expiry trails the kernel's by under a second, since the
  kernel counts from its current whole second). If the agent cannot take
  the grant, the kernel grant is revoked and `Add` fails, so a sandbox never
  misses a grant the host sees.
- `Read` and `Lookup` try the kernel first and fall back to the agent;
  `List` merges both by name (the kernel's expiry wins); `Revoke` clears
  both. A kernel `ErrExpired` survives the fallback.
- Inside a user namespace other than the initial one (`/proc/self/uid_map`
  is not `0 0 4294967295`), broker never starts an agent: one started there
  dies with the sandbox. It uses an agent already running, or grants in the
  sandbox's own keyring alone. If keyctl itself is blocked there, the agent
  is the only store. `broker approve` usually runs outside the sandbox
  anyway (systemd-run reaches the user's service manager), so a
  `--request` grant lands on the host.
- A sandbox reaches the agent when it shares broker's runtime directory
  (`$XDG_RUNTIME_DIR/broker`, or `BROKER_HOME/run`) and allows connecting
  to a unix socket. The agent checks the peer's uid with `SO_PEERCRED`,
  which the kernel reports in the agent's namespace, so a sandboxed agent
  of the same user is answered and anyone else is not.
- The agent is started by the human's `grant` or by the approver, never by
  the agent's own process tree, and it sets `PR_SET_DUMPABLE=0` before it
  holds anything, so other processes of the user cannot ptrace it or read
  `/proc/PID/mem`.
- Rejected: a grant file under `$XDG_RUNTIME_DIR` (2026-10-22). A grant is
  the decrypted value, so that file would be plaintext any process of the
  user can read regardless of expiry, lingering past it until something
  cleans up.

## Commands

Human-facing (rare):

- `broker init --identity FILE...`
- `broker set NAME [--public|--private] [--desc TEXT] [--stdin]`: value from
  a hidden TTY prompt, or stdin with `--stdin`. New secrets default to
  private. Overwrites keep the tier and description unless told: `--public`
  or `--private` moves the secret, `--desc` replaces the description.
- `broker rm NAME`: needs a terminal, with no `--stdin` way around it, so an
  agent cannot delete secrets; also drops the secret's grant.
- `broker grant NAME... [--for DUR] | --all`
- `broker revoke NAME... | --all`
- `broker get NAME`: prints the value and a newline (`set --stdin` drops
  one, so `get | set` round trips). Public: immediate. Private: always a
  fresh touch, a grant does not satisfy it (grants are for exec only).
- `broker import FILE [--public] [--overwrite] [--dry-run]`: parse
  `export NAME=value` / `NAME=value` lines (quotes handled) from shell env
  files like `~/.zshenv`, store them, print which lines can now be deleted.
  Never prints values (see below).
- `broker backup -o FILE [--generate-passphrase] [--include-log]` /
  `broker restore FILE [--identity FILE...] [--force]` (see below)
- `broker log [--since DUR] [--name NAME] [--json]`

Agent-facing (frequent, no prompts ever):

- `broker ls [--json]`: name, tier, description, granted (bool), grant
  expiry, and `changed` for a grant whose value the vault has since
  replaced (see [Grants](#grants-kernel-keyring)). Never values.
- `broker exec NAME... -- CMD [ARGS...]`: run CMD with the named secrets added
  to its environment. Exit code, stdin, and signals pass through.
  `--request [--reason TEXT]` asks for missing grants first (below).
- `broker request NAME... [--for DUR] [--reason TEXT] [--timeout 2m]`: ask
  for grants in the request window and wait. Blocks on the human, but never
  prompts in the terminal.

## Request window

An agent in T3 or any other GUI harness runs in a terminal the human never
sees, so it can't hand them a `broker grant` to type. `request` (and
`exec --request`, which also shows the command it will run) asks them on
their desktop instead.

- `request` (agent side) resolves the names to private ones without a live
  grant (a changed grant is not live). None: done, no window. Else it
  writes the request as JSON (names, duration, reason, command, its own
  PID, a deadline) to the stdin
  of `broker approve`, started with `systemd-run --user --wait --pipe
  --collect` and an allowlisted environment (`approverEnv`), and reads one
  JSON result line from its stdout. Without systemd it starts it with
  `setsid`. No desktop (`WAYLAND_DISPLAY`/`DISPLAY` unset), no
  broker-dialog, or an approver that never answers: the same `no_grant`
  as plain exec, with the reason in `message`.
- `broker approve` (hidden) holds the unwrapped private tier identity, so
  it must not be the agent's descendant: under `kernel.yama.ptrace_scope=1`
  a process may ptrace its descendants. systemd-run makes it a child of the
  user's service manager; it also sets `PR_SET_DUMPABLE=0`, as ssh-agent
  does. On macOS (no systemd-run) it starts with `setsid`, and the hardened
  runtime keeps its memory private whatever its parent; it also sets
  `PT_DENY_ATTACH`. Being outside the agent's process group also keeps the window up
  if the agent's command is killed; the grant still lands.
- The approver checks `/proc/PID/exe` is broker before showing anything,
  and reads the requester's cwd and process chain from `/proc` itself (and
  which grants are changed from the vault and keyring), so
  the window shows where the request came from, not what the request
  claims. Only `--reason` (and the command) are the agent's word, and the
  window labels the reason as such.
- One window at a time: the approver takes an flock on
  `$XDG_RUNTIME_DIR/broker/request.lock` (or `state/` without one) until
  the deadline, then re-resolves the names, so a queued request often finds
  them granted and shows nothing.
- What approving takes follows the vault (`approveBy`): `touch` when the
  first hardware unlock is a plugin that needs a human present (yubikey,
  se, fido2-hmac: unlocking starts as the window opens, so the key blinks
  or Touch ID asks, and that is the approval), `passphrase` (the prompt
  appears in the window), or `click` for a plain vault or any other
  hardware plugin (a TPM without a PIN unlocks with nobody there), which
  waits for Approve first. `Device` names the plugin, so the window says
  "Touch your YubiKey" or "Confirm with Touch ID".
- A request window needs a desktop: `WAYLAND_DISPLAY` or `DISPLAY` on
  linux, the Aqua login session on macOS (`launchctl managername`; an ssh
  session is Background), and on Windows neither session 0 nor SSH.
  Windows shows no requester directory: reading it needs the process's
  PEB. `approverEnv` adds the Windows variables a process cannot start
  Winsock or WebView2 without (`SYSTEMROOT`, `LOCALAPPDATA`, ...). Windows
  only lets the foreground process hand the foreground on, so the window
  attaches to the foreground window's input queue to take the keyboard, or
  Escape and the passphrase field would get no keys. The page reports its
  body height and the window resizes to it either way; on linux that needs
  equal min and max size hints, since a non-resizable GTK3 window on
  Wayland never shrinks. A wrong passphrase is asked for again
  with the error showing; other unlock failures offer Try again; three
  failures end the request. The human may pick a shorter duration (the
  asked one, and 5m, 15m, 1h, 4h below it) until the grant is written.
- Outcomes: granted (exit 0, one `granted NAME for DUR (until ...)` line
  per name, on stderr for exec), `denied` (exit 77: Deny, Escape, or
  closing the window), `request_timeout` (exit 75, with `ask_human`).
  The approver appends one `request` audit entry under the requester's
  PID, cwd, and chain, with the duration and reason.
- `internal/approval` is the protocol and session. broker-dialog only
  carries JSON lines: broker's messages (`request`, `prompt`, `status`,
  `done`) go to the page as a JS string literal for `JSON.parse`, the
  page's replies (`approve`, `deny`, `value`, `for`) come back on stdout,
  which it moves to a private fd so GTK and WebKit output can't corrupt
  it. The page sets every string with `textContent`, has a CSP with no
  network, and can't navigate; devtools and the context menu are off.
  Fixed size, app id `broker`, so tiling compositors float it.
  `WEBKIT_DISABLE_DMABUF_RENDERER=1` avoids a Wayland protocol error on
  NVIDIA with explicit sync.

## Phone page

A human driving agents from a phone (T3 on the couch) cannot see the
request window. `internal/phone` serves the same request on an HTTPS page
on the local network, and a security key plugged into the phone approves
it.

- The key never signs for broker. Enrollment makes a non-discoverable
  WebAuthn credential (UV discouraged, so touch only) and evaluates the
  PRF extension (CTAP `hmac-secret`) on a random 32-byte salt. HKDF-SHA256
  over that output (info `broker phone unlock v1`) is an X25519 scalar,
  turned into an ordinary age identity through its Bech32 text form, since
  age has no raw-bytes constructor. The private tier is wrapped to its
  recipient beside the other unlocks (`Vault.SetUnlocks`, file before
  config), so the vault stays plain age. Approving evaluates PRF again,
  which takes a touch, and the page posts the output back over TLS.
- The server derives the identity and compares its recipient with the one
  enrolled for the phone that sent it before anything reaches the vault;
  a wrong key gets 403. The approver then unwraps with it
  (`Vault.UnlockWith`).
- Config: `phone` holds the page (`origin`, `listen`, `cert`, `key`), and
  each phone unlock holds its name, credential ID, salt, the SHA-256 of
  its page token, and its push subscription. The relying party ID is the
  origin's host, so changing the origin means enrolling again. The cert
  and key are read at every start, so a renewed certificate needs nothing.
- The page listens only while an approver or `enroll` runs, on `listen`.
  The approver starts it beside the window when a phone is enrolled
  (`approval.Remote`: the page gets every message the window gets, its
  `deny` and `for` replies count like the window's, and the page going
  away is not a deny). `unlockApproved` waits on the window's unlock and
  the phone at once; the first to unlock wins, and the phone's chosen
  duration is applied with its unlock so reply order cannot change it.
  With no desktop the phone alone is enough, so `requestWindow` passes
  when a phone is enrolled. On `Finish` the page gets up to 2s to fetch
  the outcome before it stops.
- API: the page polls `GET /api/poll?after=N` (long poll, 25s) and posts
  `/api/reply` and `/api/unlock`. Every call carries the phone's token as a
  bearer token (kept in the page's localStorage, which has no 400-day cap
  as cookies do), and writes must carry the page's `Origin`. Enrollment
  uses a one-time token in the link's fragment, which the browser never
  sends, and the page drops it from the address bar. Responses carry a CSP
  allowing only the origin, `frame-ancestors 'none'`, and `no-store`.
- Notifications are Web Push (`internal/webpush`): RFC 8291 aes128gcm to
  the subscription's keys, signed with a VAPID key (RFC 8292) kept beside
  the config so a restore does not orphan the subscriptions. The push
  service (Google's, for Chrome) sees only ciphertext, and the payload
  only says how many secrets are wanted, since it shows on the lock
  screen. TTL is the request's deadline, `Urgency: high`, `Topic` so a
  newer request replaces a waiting one. A tap opens the page's own origin,
  never a URL from the payload. The service worker caches the page so a
  late tap says nothing is pending instead of showing a browser error.
- `enroll` and `phone rm` need a terminal, like every command that changes
  the vault, and unlock with the vault's own unlock (a touch). The key
  the phone used is often that same YubiKey, so after the phone's part
  `enroll` waits for Enter before unlocking, and offers a retry if the
  unlock fails. The page keeps its new token only once the PC says the
  vault is wrapped (`GET /api/enroll/result`, authorized by that token),
  so it never claims an enrollment the PC did not finish. `rm`
  refuses to remove the last unlock.
- Threat: the page's TLS key is a file the user can read. Something running
  as the user that takes it and the port can serve a look-alike page and
  receive the PRF output from a touch meant for a real request: the fake
  window again, inside this design's threat model. Other devices on the
  network have neither the TLS key nor a phone token.

## Android app

Android's WebAuthn (Play services) runs user verification on every
security key, whatever the page asks for, so the page in Chrome costs a
PIN per approval. `android/` is a native app that speaks the page's API
and talks to the key itself.

- Key: YubiKit (`Ctap2Session` over FIDO HID on USB, or the smart card
  interface on NFC). Enrollment makes a non-discoverable ES256 credential
  with `hmac-secret` (`rk` false, no `pinUvAuthParam`; YubiKeys allow that
  with a PIN set, `makeCredUvNotRqd`) and evaluates it in the same
  session. Approving evaluates `hmac-secret` with only user presence: key
  agreement through `ClientPin` (protocol 2 when offered), the salt
  encrypted and MACed with the shared secret, the output decrypted. The
  salt goes through WebAuthn PRF's `SHA-256("WebAuthn PRF" || 0 || salt)`,
  so the server side is the page's unchanged. A UV-less evaluation reads
  the key's no-UV secret (CTAP 2.1 keeps two), so a page credential made
  with a PIN gives other output here, and the app enrolls its own.
- API: the same endpoints as the page. The app sends no `Origin`; writes
  accept a missing one, since browsers always send it on POST, and every
  write still needs the phone's token. `POST /api/enroll` carries `app`
  and the app's FCM token instead of a push subscription. The enroll link
  is the page's (`/enroll#TOKEN`), scanned from the QR code with Play
  services' code scanner.
- Notifications are FCM HTTP v1 data messages (`internal/fcm`): an RS256
  JWT from the service account key traded for an access token per send
  (each approver is a new process), `priority: HIGH`, TTL to the
  deadline, one collapse key. FCM payloads are readable by Google, so they
  carry only `type` (`request`, `done`, `enrolled`), a random request
  `id`, `count`, and `deadline`. On `request` the app fetches the details
  from the page (one `poll?after=0`, 5s) and posts a notification whose
  private version names the secrets and whose public (lock screen) version
  only counts them, with a countdown, Approve (opens the app and starts
  the key flow), and Deny (a broadcast that posts `/api/reply`). When the
  approver finishes it sends `done` for that `id` (3s cap), which clears
  the notification; the app remembers done ids for an hour, so a `done`
  that FCM delivers first still wins.
- Firebase is set up at run time from the project's public settings,
  which `broker phone fcm` reads from `google-services.json` into
  `config.fcm` and the page hands the app at enrollment. The APK carries
  no project, and the google-services plugin is not used. The service
  account key stays beside the config (`fcm-service-account.json`, 0600),
  readable by the user like the VAPID key; with it someone can send the
  app notifications, which only make it ask the PC what is pending. A
  token FCM later replaces is not sent anywhere (the page only listens
  during requests); the app says notifications stopped and to enroll
  again.
- The app keeps the origin, its phone token, and the Firebase settings in
  private preferences, excluded from backup and device transfer: a
  restored copy would hold a token for a phone that never enrolled.
- Release: `release.yml` builds the APK with the tag's version
  (`-PbrokerVersion`, version code MAJOR*10000 + MINOR*100 + PATCH) and
  signs it with one keystore kept in repository secrets, since Android
  only upgrades an app signed by the key it was installed with. Anyone
  holding that keystore can ship an update the phone accepts; the
  enrollment it would inherit still needs the YubiKey for every approval.

## exec behaviour

- Resolve each NAME: public from disk, private from the keyring. Any private
  NAME without a live grant fails before running anything: exit 77 and
  print to stderr one JSON line:
  `{"error":"no_grant","names":["X"],"ask_human":"broker grant X --for 1h"}`.
  Names whose grant holds a changed value are also listed in `changed`,
  with a `message` saying so, since the agent was using them a moment ago.
  Unknown names: exit 78, `{"error":"unknown_secret","names":[...]}`.
- Redaction: when the child's stdout or stderr is not a TTY (the agent case),
  pipe it through a streaming redactor (`internal/redact`) that replaces
  every private value with `[REDACTED:NAME]`, holding back at most
  `maxlen-1` bytes so matches across read boundaries are caught. Only bytes
  that could still start a value are held, so ordinary output (prompts,
  progress lines) is not delayed; the held tail is flushed at EOF. At each
  position the longest value wins; overlapping matches merge into one marker
  named for the leftmost, so no byte of any occurrence leaks. Skip values
  shorter than 6 bytes. When the stream is a TTY (a human), pass it through
  untouched so interactive tools work.
- Never put values in argv, never write them to disk.
- stdin goes to the child as is (a terminal stays a terminal). Broker
  catches SIGINT, SIGQUIT, SIGTERM, SIGHUP, SIGUSR1, and SIGUSR2 so it
  outlives the child (its output still needs redacting) and forwards them.
  The exception is SIGINT/SIGQUIT while broker is in its terminal's
  foreground process group: the terminal already sent them to the child,
  and a second Ctrl-C makes some tools force quit. Signals broker started
  out ignoring stay ignored. SIGTSTP/SIGCONT keep their defaults so job
  control works.
- Broker exits with the child's status, or 128+N if signal N killed it,
  writing nothing of its own. A command that cannot be found fails with
  `command_not_found` (exit 127), one that cannot be started with
  `cannot_exec` (exit 126), as a shell would.

## Audit log

One JSON object per line: `ts`, `action` (`init|set|rm|grant|revoke|get|exec|exit|import|backup|restore|request|phone-enroll|phone-rm|phone-fcm`),
`names`, `tier`, `for` (a grant's duration), `ok`, `error`, `cwd`, `cmd`
(exec's child argv[0] only), `status` and `took` (exit's child status and
run time), `pid`, and `ancestry`: the process chain from
`/proc/<pid>/stat` + `comm` up to pid 1, e.g.
`["broker","bash","claude","zsh","alacritty"]`, so it shows which agent
asked. Never log values. Log failures too (a denied `no_grant` is the most
useful line in the file).

- `tier` is the names' tier, `mixed` when they span both, and absent when
  none is known (an unknown name). `error` is the agenterr code. Empty
  `names`, `tier`, `for`, `error`, `cmd`, `status`, and `took` are left
  out.
- `ls` and `log` are not logged. Flag and argument errors cobra catches
  before a command runs are not logged; everything after is.
- import logs the names it stored; `import --dry-run` is not logged.
- grant logs the private secrets it grants (by name, also with `--all`),
  so a grant of only public secrets logs no names and no `for`; revoke
  logs the names it revoked, with their tier, and none that had no grant. backup and restore log every secret
  the backup carries (tier usually `mixed`), so `--name` finds the backups
  holding a secret.
- exec logs before the child starts (it may run for hours): `ok` means
  the secrets were handed to the child. Once the secrets were handed out,
  an `exit` entry follows when the child is done, with the exec's names,
  tier, cmd, and pid (they pair by pid): `status` as a shell reports it
  (128+N when signal N killed it) and `took` (to the millisecond under a
  second, to the second above). `ok` is true for status 0; any other
  status is the child's outcome, not a broker failure, so it has no
  `error`. A child that could not start (`command_not_found`,
  `cannot_exec`) or whose output could not be copied gets that `error`
  and no `status`. Signals sent to broker are forwarded and broker
  outlives the child, so only a broker killed outright (SIGKILL, a crash,
  power loss) leaves an exec without its exit. That is the point of the
  pair: an exec with no exit is either still running or had broker killed
  under it, never a command that quietly finished or failed.
- Each line is one `O_APPEND` write under an exclusive `flock`, so
  concurrent brokers never interleave lines. The lock wait is bounded (1s)
  so a stopped broker cannot stall the rest; after that the write goes
  ahead unlocked. A line cut short (full disk) is skipped on read and does
  not swallow the next one.
- Logging is best effort: if the line cannot be written, broker prints
  `broker: audit log not written: ...` on stderr (before any JSON error
  line) and the command's outcome is unchanged.
- `broker log` prints a table (time, action, result, tier, names, exec's
  cmd or grant's duration, and the ancestry above broker). `--since DUR`
  keeps entries newer than DUR, `--name NAME` those naming NAME, and
  `--json` prints the matching lines as written.
- The table also shows what it can work out from the whole log, filters
  applied after:
  - `expire` rows: grants run out in the kernel (or the grant agent) with
    no broker running to log them, so the table computes them. A grant or
    request entry that is `ok` with a `for` gives each of its names an
    expiry at `ts` + `for`, unless a later grant, request, revoke, or rm
    naming it comes first (a new grant replaces the old). An expiry goes
    after any entry with the same time. Grants still live have no row;
    `--json` never has these rows.
  - `(running)` after an exec's cmd: it handed out its secrets, no exit
    with its pid follows, and its pid is a live process with broker's
    name (not this one). A reused pid can be marked by mistake; an exec
    with no exit and no live broker is one whose broker was killed.

## Import

- `broker import FILE` reads FILE without running it (`internal/envfile`)
  and never edits it. It understands the shell subset env files use for
  plain values: an optional `export`, single quotes, double quotes (where
  `\` escapes only `$`, `` ` ``, `"`, `\`, and newline), backslash escapes,
  line continuations, quoted values spanning lines, a trailing `;`, and
  trailing comments. Other statements (`export NAME` with no value,
  commands, `alias`, and assignments by `typeset`, `declare`, `readonly`)
  are ignored without a report, quotes and all, so a quoted string
  spanning lines is never read as assignments.
- An assignment is skipped and reported (line, name, reason) when its value
  needs the shell: a variable reference, command substitution, arithmetic,
  `$'...'`, a tilde at the start or after a `:`, `+=`, or more than one word
  (`A=1 B=2`, arrays, `&&`). Also skipped: names that are not valid secret
  names, values `set` would refuse (empty, too long, NUL), and names already
  in the vault unless `--overwrite`. Nothing prompts, so values from the
  file never need a terminal.
- The last assignment of a name wins, as in the shell. When it is skipped,
  earlier ones are too ("set again on line N"); when it is imported,
  earlier plain assignments of the name are deletable with it.
- Tiers follow `set`: new secrets are private, `--public` makes (or moves)
  them public, and `--overwrite` otherwise keeps an existing secret's tier
  and description.
- Output: the imported names with tier and lines, the skipped lines, and
  `safe to delete from FILE: lines 2-5, 11`. An imported name that a
  statement left in FILE may still expand is listed as kept, with the lines
  using it, and its lines are not offered for deletion. That check is
  conservative: any `$NAME` or `${NAME}` in a skipped assignment or other
  statement counts (comments and plain NAME=value assignments expand
  nothing). When storing fails partway, the report covers what was stored
  and the error follows on stderr.
- `--dry-run` prints the same report ("would import") and stores and logs
  nothing. A real import logs one entry naming the secrets it stored, with
  their tier (`mixed` when `--overwrite` kept both); an import that stores
  nothing still logs one entry without names.

## Backup and restore

- `broker backup -o FILE [--generate-passphrase] [--include-log]`: one
  touch unwraps the private identity, and FILE gets the whole vault with
  that identity *unwrapped*, encrypted to an age scrypt passphrase
  recipient alone (scrypt must be the only recipient in an age file, so
  this is a separate file from the vault). FILE is replaced atomically at
  0600. The passphrase is typed twice, with a warning under 20 characters;
  `--generate-passphrase` instead makes 7 words from the EFF large
  wordlist (embedded, about 90 bits) and prints them on stderr. There is
  no passphrase env var or fd: tests swap the prompt in process.
- Format: an uncompressed tar laid out like `BROKER_HOME`, so the stock
  tools recover it (below). `manifest.json` comes first:
  `{"format":1,"created":...,"broker_version":...,"log":bool}`. Then
  `config/config.json`, `data/public.key`, `data/private.key` (the
  unwrapped private identity in age-keygen format),
  `data/secrets/<tier>/<NAME>.age` and `.json`, and with `--include-log`
  `state/log.jsonl`. Directories are 0700 and files 0600 in the tar.
  Restore reads every format up to its own and refuses newer ones with a
  clear error; a later format bumps the number and keeps reading old ones.
- `broker restore FILE [--identity FILE...] [--insecure-plain-identity]
  [--force]`: refuses an existing vault without `--force` (before any
  prompt), asks for the passphrase, and wraps the private identity to the
  `--identity` files, or to the unlock identities in the backup's config
  (same YubiKey travels with you). Plain identities need
  `--insecure-plain-identity`, as for init; plain ones already in the
  backup's config do not.
- Restore never leaves a half vault. It unpacks into a staging directory
  beside the data directory, accepting only the entries above (no other
  names, no `..`, no links), checks both tier keys against the config's
  recipients, decrypts every secret, and rewraps the private identity, all
  before touching the live vault. Then it removes config.json, swaps the
  data directory for the staged one with one rename, and writes the new
  config.json. A failure up to there puts the old vault back; an
  interruption leaves no config.json (`no_vault`), and rerunning
  `restore --force` finishes the job. The unwrapped identity never touches
  disk.
- The local audit log is never rewritten. A backup's log becomes
  `log.jsonl` only when there is none (or it is empty, a fresh machine);
  otherwise it is kept beside it as `log.restored-<backup time>.jsonl`.
  The restore entry is then appended to the local log.
- Replacing a vault revokes every grant: they hold the old vault's values
  under names the restored vault reuses. Best effort, like `rm`.
- The backup file is safe to keep anywhere (git, R2, NAS). Its strength is
  the passphrase; offline guessing has no rate limit.
- Recovery without broker:
  `mkdir r && age -d FILE | tar -x -C r`, then
  `age -d -i r/data/private.key r/data/secrets/private/NAME.age` (or
  `public.key` for the public tier).

## Errors and output for agents

- Every failure an agent can hit has a stable JSON line on stderr with an
  `error` code and, where a human must act, an `ask_human` command string.
- `--json` on read commands for machine output; human output otherwise.
- No command an agent runs ever blocks on a terminal prompt: if a prompt
  would be needed and stdin is not a TTY, fail with
  `{"error":"needs_human",...}`. `request` blocks on the human only through
  the request window, up to its timeout.

## Testing

- `BROKER_HOME` in a temp dir plus a plain identity
  (`--insecure-plain-identity`) covers everything but the touch itself.
- Keyring tests rerun the test binary in a new user namespace mapping only
  the caller's uid (`keyringtest.Main`), started from a fresh session keyring
  (`KEYCTL_JOIN_SESSION_KEYRING`). A session keyring alone does not isolate
  the user keyring, which is per uid; a user namespace has its own, so tests
  never touch the real one and leave nothing behind even if they crash.
  Without user namespaces the keyring tests skip.
- On macOS and Windows the same suite runs against real grant agents: each
  test's `BROKER_HOME` has its own socket, so `keyringtest` isolates by
  doing nothing, and `keyring.AgentCommand` starts the test binary as
  broker. `TestGrantd` runs that path on linux too. On linux the tests run
  in a user namespace, so the store never starts an agent itself; the
  `TestStore*` tests pair an isolated kernel keyring with an in-process
  agent. On Windows the cmd
  tests end those agents at exit (`stopTestAgents`), since go test cannot
  delete a running copy of the test binary. Unix permission bits are not
  checked on Windows, where privacy is the profile directory's ACL.
- Request tests run a real `broker approve` subprocess (the test binary)
  against a fake window (the test binary again) that acts out a script
  and logs what it was shown. broker-dialog itself is checked by hand.
- Run Go builds and tests under
  `systemd-run --user --scope -q -p MemoryMax=4G nice -n 19 go test ./...`.

## Out of scope for v2

Server mode, multi-user, scopes/ACLs, macOS/Windows (Keychain / Touch ID
via `age-plugin-se` is a natural later port), credential-injecting proxy,
rotation.

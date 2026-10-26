---
name: broker
description: Credentials through the `broker` CLI. Use whenever a task needs an API key, token, password, or other credential, or a command fails for missing auth.
---

# broker

`broker` holds the human's secrets. You run commands **with** a secret; you never read it. Secret names are environment variable names, and `broker exec` puts them in the child's environment. `ls`, `exec`, and `request` are yours; every other subcommand is the human's.

## Use a secret

1. **Discover**: `broker ls --json` prints an array of `{"name","tier","description","granted","expires",...}`. Pick by name and description.
   - `public`: always usable.
   - `private`: usable while `"granted": true`; otherwise ask for a grant with `--request` (step 2).
2. **Run**: `broker exec --request --reason "WHY" NAME... -- CMD [ARGS...]`. With grants live it just runs. With any missing, it opens a window on the human's desktop showing the names, CMD, and your reason, waits up to 2 minutes for them to approve, then runs. Give the shell command a timeout of at least 150 seconds. Write the reason for the human: one plain sentence on what the task needs the secret for. A tool that reads the variable itself runs directly:

   ```sh
   broker exec --request --reason "list your repos for the audit" GH_TOKEN -- gh repo list
   ```

   To reference the variable in arguments, expand it inside the child with `sh -c` and single quotes:

   ```sh
   broker exec --request --reason "check which account the token is for" GITHUB_TOKEN -- sh -c 'curl -sf -H "Authorization: Bearer $GITHUB_TOKEN" https://api.github.com/user'
   ```

   Written straight into the arguments (`-- curl -H "Authorization: Bearer $GITHUB_TOKEN" ...`), the variable expands in your own shell first, where it is empty.
3. **Read the result** by exit status and stderr (next section).

Private values in the child's stdout and stderr come back as `[REDACTED:NAME]`. That is expected: the command saw the real value and worked. Leave the value unrecovered. Public values are not redacted, and `broker get NAME` prints a public one.

Keep each value inside the exec'd process: run `broker exec` per command (not `export` into your shell), and keep values out of files, commits, logs, and chat. Redaction covers only the child's output streams, not files it writes. A private `broker get` needs a hardware touch and would print the value into your transcript; use `exec`.

If a credential the task needs is missing from `ls`, ask the human to store it (`broker set NAME --desc "what it is for"`) rather than paste it to you.

## Results

`exec` exits with the child's own status. Broker's own failures happen before the child runs and always print exactly one JSON line on stderr with an `error` field; that line, not the number, tells them apart from the child's.

```json
{"error":"no_grant","names":["GITHUB_TOKEN","DB_PASS"],"ask_human":"broker grant GITHUB_TOKEN DB_PASS --for 1h"}
```

| exit | `error` | meaning | do |
|---|---|---|---|
| child's | (none) | the command ran | treat it as the command's result |
| 77 | `no_grant` | private secret not granted, or the grant expired | without `--request`: rerun with it. With it, the `message` says there is no request window (no desktop, or broker-dialog missing): ask the human to run `ask_human` verbatim; retry after they confirm |
| 77 | `denied` | the human turned the request down | stop; tell the human what is blocked and why; request again only if they say so |
| 75 | `request_timeout` | nobody answered the window | tell the human the request is waiting, with `ask_human` as the terminal alternative; retry once they reply |
| 78 | `unknown_secret` | `names` are not in the vault | recheck `broker ls --json`; if absent, ask the human to `broker set` it |
| 75 | `needs_human` | you ran a human command (`get` private, `grant`, `set`, `rm`, ...) | ask the human to run `ask_human` |
| 66 | `no_vault` | broker is not set up | ask the human to run `ask_human` |
| 127, 126 | `command_not_found`, `cannot_exec` | CMD after `--` is missing or not executable | fix CMD |
| 64 | `usage` | bad arguments (often a missing `--`) | fix the call |

## Sandboxes

On Linux, grants live in the kernel user keyring, which is per user namespace, and in broker's grant agent, which a sandbox reaches through a unix socket in `$XDG_RUNTIME_DIR/broker`. Inside a sandbox (bubblewrap and sandboxes built on it) grants usually still work through the agent. If the human says they granted a name but `broker ls --json` still shows `"granted": false` (or `exec` keeps failing with `no_grant`, or `ls` prints `grants not shown: ...` on stderr), your sandbox probably hides that directory or blocks unix sockets. Granting again will not help: tell the human, and either ask them to share `$XDG_RUNTIME_DIR/broker` with the sandbox or run the `broker exec` command outside it, requesting unsandboxed execution or asking permission to.

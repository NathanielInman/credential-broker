---
name: broker
description: Credentials through the `broker` CLI. Use whenever a task needs an API key, token, password, or other credential, or a command fails for missing auth.
---

# broker

`broker` holds the human's secrets. You run commands **with** a secret; you never read it. Secret names are environment variable names, and `broker exec` puts them in the child's environment. `ls` and `exec` never prompt; every other subcommand is the human's.

## Use a secret

1. **Discover**: `broker ls --json` prints an array of `{"name","tier","description","granted","expires",...}`. Pick by name and description.
   - `public`: always usable.
   - `private`: usable while `"granted": true`. If the names you need are not granted, run `broker exec A B -- true` to get one `no_grant` line asking for all of them at once (one touch for the human).
2. **Run**: `broker exec NAME... -- CMD [ARGS...]`. A tool that reads the variable itself runs directly:

   ```sh
   broker exec GH_TOKEN -- gh repo list
   ```

   To reference the variable in arguments, expand it inside the child with `sh -c` and single quotes:

   ```sh
   broker exec GITHUB_TOKEN -- sh -c 'curl -sf -H "Authorization: Bearer $GITHUB_TOKEN" https://api.github.com/user'
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
| 77 | `no_grant` | private secret not granted, or the grant expired | stop; ask the human to run `ask_human` verbatim; retry after they confirm |
| 78 | `unknown_secret` | `names` are not in the vault | recheck `broker ls --json`; if absent, ask the human to `broker set` it |
| 75 | `needs_human` | you ran a human command (`get` private, `grant`, `set`, ...) | ask the human to run `ask_human` |
| 66 | `no_vault` | broker is not set up | ask the human to run `ask_human` |
| 127, 126 | `command_not_found`, `cannot_exec` | CMD after `--` is missing or not executable | fix CMD |
| 64 | `usage` | bad arguments (often a missing `--`) | fix the call |

## Sandboxes

Grants live in the kernel user keyring, which is per user namespace. If the human says they granted a name but `broker ls --json` still shows `"granted": false` (or `exec` keeps failing with `no_grant`, or `ls` prints `grants not shown: ...` on stderr), you are probably in a sandbox with its own user namespace (bubblewrap and sandboxes built on it). Granting again will not help: run the `broker exec` command outside the sandbox, requesting unsandboxed execution for it or asking the human for permission to.

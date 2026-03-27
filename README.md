# awn

TUI automation for AI agents. A daemon that manages headless terminal sessions so AI agents can screenshot, send input, and wait on terminal state.

Recreates the core of [agent-tui](https://github.com/pproenca/agent-tui) in Go.

<img src="https://skillicons.dev/icons?i=go" alt="Go" />

## Usage

```bash
make build              # bin/awn + bin/awnd
awnd &                  # start daemon on 127.0.0.1:7600

awn create bash         # start a session
awn screenshot <id>     # capture screen
awn input <id> "ls\n"   # send keystrokes
awn wait <id> "done"    # block until text appears
awn close <id>          # terminate session
```

## RPC Methods

JSON-RPC 2.0 over WebSocket.

| Method | Params | Returns |
|--------|--------|---------|
| `create` | `{command, args?, rows?, cols?}` | `{id}` |
| `screenshot` | `{id}` | `{rows, cols, lines, cursor}` |
| `input` | `{id, data}` | `null` |
| `wait_for_text` | `{id, text, timeout_ms?}` | `null` |
| `wait_for_stable` | `{id, stable_ms?, timeout_ms?}` | `null` |
| `close` | `{id}` | `null` |
| `list` | none | `{sessions: [id...]}` |

## Auth

Set `AWN_TOKEN` on both daemon and CLI for Bearer token auth. Connections with a non-empty `Origin` header are rejected.

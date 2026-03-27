# awn

TUI automation for AI agents, written in Go. Recreates the core functionality of [agent-tui](https://github.com/pproenca/agent-tui) — a daemon that manages headless terminal sessions so AI agents can screenshot, send input, and wait on terminal state.

## Architecture

```
AI Agent ──► awn CLI ──► WebSocket JSON-RPC 2.0 ──► awnd daemon
                                                       │
                                                  Session Manager
                                                  (goroutine per session)
                                                       │
                                                  PTY + Terminal Emulator
                                                       │
                                                  TUI App (bash, htop, vim...)
```

- **awnd** — background daemon managing concurrent PTY sessions, serves JSON-RPC 2.0 over WebSocket on `127.0.0.1:7600`
- **awn** — thin CLI client that connects to the daemon

## Project Layout

```
cmd/awnd/           Daemon entrypoint
cmd/awn/            CLI client entrypoint
internal/screen/    Snapshot types (screen buffer model)
internal/session/   PTY lifecycle, session manager (create/screenshot/input/wait/close)
internal/rpc/       JSON-RPC method dispatch
internal/transport/ WebSocket server
```

## Build & Run

```bash
make build          # builds bin/awn and bin/awnd
make run            # builds and starts the daemon
make test           # runs all tests with race detector
make clean          # removes bin/
```

## Dependencies

- `github.com/creack/pty` — PTY management
- `github.com/gorilla/websocket` — WebSocket transport
- `github.com/google/uuid` — session IDs

## RPC Methods

| Method | Params | Returns |
|--------|--------|---------|
| `create` | `{command, args?, rows?, cols?}` | `{id}` |
| `screenshot` | `{id}` | `{rows, cols, lines, cursor}` |
| `input` | `{id, data}` | `null` |
| `wait_for_text` | `{id, text, timeout_ms?}` | `null` |
| `wait_for_stable` | `{id, stable_ms?, timeout_ms?}` | `null` |
| `close` | `{id}` | `null` |
| `list` | none | `{sessions: [id...]}` |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AWN_ADDR` | Daemon address for the CLI | `ws://localhost:7600` |
| `AWN_TOKEN` | Bearer auth token (shared between client and daemon) | *(none)* |

## Authentication

Set `AWN_TOKEN` on both the daemon and CLI to enable Bearer token authentication. The daemon rejects WebSocket upgrades without a valid `Authorization: Bearer <token>` header when a token is configured.

The daemon also rejects connections with a non-empty `Origin` header to prevent browser-based cross-origin attacks.

## Known Limitations

- The terminal parser in `session/manager.go` (`readLoop`) is simplified — handles basic characters, `\n`, `\r` but not full ANSI/VT100 escape sequences. For real TUI apps (htop, vim, ncurses), swap in `github.com/taigrr/bubbleterm` for proper terminal emulation. The session manager interface stays the same.

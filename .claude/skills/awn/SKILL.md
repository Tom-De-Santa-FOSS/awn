---
name: awn
description: "TUI automation for AI agents — manage headless terminal sessions, take screenshots, send input, and wait for output. Use when the user needs to interact with terminal applications programmatically, says /awn, or wants to automate a TUI. Also trigger on: 'terminal automation', 'TUI session', 'screenshot terminal', 'headless terminal', 'awnd', 'PTY session'."
trigger: user-invocable
---

You are operating awn, a TUI automation daemon. Read `$ARGUMENTS` to understand what the user needs. Start the daemon if not running, then execute the requested operations.

## Quick Start

```bash
awnd &                              # Start daemon (localhost:7600)
awn create bash                     # Create session
awn screenshot <id>                 # Capture terminal state
awn input <id> "ls -la\n"          # Send input
awn wait-for-text <id> "done"      # Wait for text to appear
awn close <id>                      # End session
```

## Commands

| Command | Description |
|---------|-------------|
| `awn create <cmd> [args...]` | Start new PTY session |
| `awn screenshot <id>` | Capture screen buffer |
| `awn input <id> "<data>"` | Send keys/text to session |
| `awn wait-for-text <id> "<text>"` | Block until text appears |
| `awn wait-for-stable <id>` | Block until screen stops changing |
| `awn close <id>` | Terminate session |
| `awn list` | List active sessions |

## Environment Variables

- `AWN_ADDR` — Daemon address (default: `ws://localhost:7600`)
- `AWN_TOKEN` — Bearer token for authentication (optional)

## RPC Methods

JSON-RPC 2.0 over WebSocket at `ws://localhost:7600`.

| Method | Params | Returns |
|--------|--------|---------|
| `create` | `{command, args?, rows?, cols?}` | `{id}` |
| `screenshot` | `{id}` | `{rows, cols, lines, cursor}` |
| `input` | `{id, data}` | `null` |
| `wait_for_text` | `{id, text, timeout_ms?}` | `null` |
| `wait_for_stable` | `{id, stable_ms?, timeout_ms?}` | `null` |
| `close` | `{id}` | `null` |
| `list` | none | `{sessions: [id...]}` |

## Workflow

1. **Start daemon** — `awnd &` (binds to `127.0.0.1:7600`, localhost only)
2. **Create session** — `awn create bash` returns a session `{id}`
3. **Interact** — Send input, take screenshots, wait for output
4. **Clean up** — `awn close <id>` when done

## When to Use

- Automating interactive terminal applications (htop, vim, ncurses)
- Taking screenshots of terminal state for AI agent vision
- Sending keystrokes to running TUI programs
- Waiting for specific output before proceeding
- Running headless terminal sessions for CI or testing

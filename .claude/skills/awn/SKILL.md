---
name: awn
description: "TUI automation daemon. Manage headless terminal sessions, take screenshots, detect UI elements, send input, wait for output. Use for terminal app automation, TUI testing, and AI agent vision of terminal state."
---

You are operating awn, a TUI automation daemon. Read `$ARGUMENTS` to understand what the user needs. Start the daemon if not running, then execute the requested operations.

## Quick Start

```bash
awnd &                              # Start daemon (localhost:7600)
awn create bash                     # Create session (returns JSON {id})
awn screenshot <id>                 # Capture terminal screen state
awn screenshot <id> --json          # Capture as structured JSON
awn detect <id>                     # Detect UI elements (accessibility tree)
awn input <id> "ls -la\n"          # Send input to session
awn wait <id> "done"               # Wait for text to appear
awn close <id>                      # End session
awn list                            # List active sessions
```

## Commands

| Command | Description |
|---------|-------------|
| `awn create <cmd> [args...]` | Start new PTY session, returns `{id}` |
| `awn screenshot <id> [--json]` | Capture screen buffer (text lines, cursor position) |
| `awn detect <id>` | Detect UI elements as an accessibility tree |
| `awn input <id> "<data>"` | Send keys/text to session |
| `awn wait <id> "<text>"` | Block until text appears on screen |
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
| `detect` | `{id}` | `{elements: [{Type, Label, Bounds, Focused}]}` |
| `input` | `{id, data}` | `null` |
| `wait` | `{id, text, timeout_ms?}` | `null` |
| `close` | `{id}` | `null` |
| `list` | none | `{sessions: [id...]}` |

## Workflow

1. **Start daemon** — `awnd &` (binds to `127.0.0.1:7600`, localhost only)
2. **Create session** — `awn create bash` returns a session `{id}`
3. **Interact** — Send input, take screenshots, detect UI elements, wait for output
4. **Clean up** — `awn close <id>` when done

## Output Formats

**screenshot** returns lines of text representing the terminal buffer:
```json
{"rows":24, "cols":80, "lines":["$ ls", "file.txt", "..."], "cursor":{"Row":1,"Col":2}}
```

**detect** returns an accessibility tree of UI elements:
```json
{"elements":[{"Type":"button","Label":"OK","Bounds":{"Row":5,"Col":10,"Width":4,"Height":1},"Focused":true}]}
```

## When to Use

- Automating interactive terminal applications (htop, vim, ncurses)
- Taking screenshots of terminal state for AI agent vision
- Detecting UI elements in TUI applications via accessibility tree
- Sending keystrokes to running TUI programs
- Waiting for specific output before proceeding
- Running headless terminal sessions for CI or testing

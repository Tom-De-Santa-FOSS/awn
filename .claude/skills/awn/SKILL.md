---
name: awn
description: "TUI automation daemon. Manage headless terminal sessions, take screenshots, detect UI elements, send input, wait for output. Use for terminal app automation, TUI testing, and AI agent vision of terminal state."
allowed-tools: Bash(awn:*), Bash(awnd:*)
---

# TUI Automation with awn

Headless terminal session management via a background daemon. Manage PTY sessions — screenshot, send input, detect UI elements, and wait on terminal state. JSON-RPC 2.0 over WebSocket.

## Core Workflow

Every terminal automation follows this pattern:

1. **Start daemon**: `awn daemon start`
2. **Create session**: `awn create <command>` (returns `{id}`)
3. **Interact**: Screenshot, send input, detect elements, wait for output
4. **Clean up**: `awn close <id>`

```bash
awn daemon start
awn create bash
# Output: {"id":"abc123"}

awn screenshot abc123                # capture screen as text
awn type abc123 "ls -la"             # send literal text
awn press abc123 Enter               # send named key
awn wait abc123 --text "$"           # wait for prompt
awn screenshot abc123 --full         # screen + detected elements
awn close abc123
```

## Command Chaining

Commands can be chained with `&&` in a single shell invocation. The daemon persists between commands, so chaining is safe and more efficient than separate calls.

```bash
# Chain create + type + press + wait in one call
awn create bash && awn type $ID "ls -la" && awn press $ID Enter && awn wait $ID --stable

# Multi-step interaction
awn type $ID "cd /tmp" && awn press $ID Enter && awn wait $ID --text "$" && awn screenshot $ID
```

**When to chain:** Use `&&` when you don't need to read the output of an intermediate command before proceeding. Run commands separately when you need to parse output between steps (e.g., screenshot to discover UI elements, then interact).

For complex multi-step workflows, prefer `awn pipeline` — it batches steps in a single RPC call.

## Sessions

```bash
awn create <command> [args...]         # start a PTY session, returns {id}
awn list                               # show active sessions
awn close <id>                         # terminate session
awn ping                               # daemon health check
awn daemon start                       # start the daemon in background
awn daemon stop                        # stop the daemon
awn daemon status                      # check daemon status
```

## Screen Capture

```bash
awn screenshot <id>                    # render screen as text lines
awn screenshot <id> --json             # full JSON response
awn screenshot <id> --full             # screen + detected UI elements + state
awn screenshot <id> --diff             # changed rows since last screenshot
awn screenshot <id> --scrollback 100   # include scrollback history
```

Screenshot formats control what the response includes:

| Format | Lines | Elements | State | Changes |
|--------|-------|----------|-------|---------|
| *(default)* | yes | no | no | no |
| `full` | yes | yes | yes | no |
| `structured` | no | yes | yes | no |
| `diff` | no | no | yes | yes |

## Input

```bash
awn type <id> "hello world"            # send literal text (no Enter)
awn press <id> Enter                   # send a named key
awn press <id> Ctrl+C                  # send key combo
awn input <id> "raw data"              # send raw bytes/escape sequences
awn mouse-click <id> 10 12             # click at row col
awn mouse-click <id> 10 12 1           # click with button (0=left default)
awn mouse-move <id> 10 12              # move cursor to row col
```

### Supported Keys

Named keys for `awn press`: `Enter`, `Tab`, `Backspace`, `Escape`, `Space`, `Delete`, `Up`, `Down`, `Left`, `Right`, `Home`, `End`, `PageUp`, `PageDown`, `Insert`, `F1`-`F12`, `Ctrl+A`-`Ctrl+Z`, `Ctrl+[`, `Ctrl+]`, `Ctrl+\`. Single characters are sent literally.

## Waiting

```bash
awn wait <id> --text "Status"          # block until text appears
awn wait <id> --gone "Loading"         # block until text disappears
awn wait <id> --regex "v\d+\.\d+"     # block until regex matches
awn wait <id> --stable                 # block until screen stops changing
awn wait <id> --timeout 10000          # set timeout in ms (default 5000)
```

Exactly one condition must be provided per wait call. Default timeout is 5000ms. The `--stable` condition uses a 500ms threshold.

## Automation

```bash
# exec: type input + Enter, then wait for output
awn exec <id> "ls -la"                         # wait for screen to stabilize
awn exec <id> "make" --wait-text "done"        # wait for specific text instead
awn exec <id> "cargo build" --timeout 30000    # custom timeout

# pipeline: batch multiple steps as JSON
awn pipeline <id> '[
  {"type": "type", "text": "ls -la"},
  {"type": "press", "keys": "Enter"},
  {"type": "wait", "text": "$"},
  {"type": "screenshot"}
]'

# stop on first error
awn pipeline <id> '[...]' --stop-on-error
```

### Pipeline Step Types

| Step Type | Fields | Description |
|-----------|--------|-------------|
| `screenshot` | -- | Capture current screen |
| `type` | `text` | Send literal text |
| `press` | `keys` | Send named key (e.g. `Enter`, `Ctrl+C`) |
| `exec` | `input`, `timeout_ms?` | Send input + Enter, wait for stable |
| `wait` | `text?`, `stable?`, `gone?`, `regex?`, `timeout_ms?` | Wait for a condition |
| `sleep` | `ms` | Pause for N milliseconds |

## Detection

```bash
awn detect <id>                        # accessibility tree of UI elements
```

Returns elements with `Type`, `Label`, `Bounds` (Row, Col, Width, Height), and `Focused` state. Use with `awn screenshot <id> --full` to get both screen content and elements in one call.

## Other

```bash
awn resize <id> 40 120                 # resize session rows/cols
awn record <id> session.cast           # write asciicast v2 recording
```

## Common Patterns

### Shell Interaction

```bash
awn create bash
awn exec $ID "ls -la"
awn exec $ID "cd /tmp" --wait-text "$"
awn exec $ID "pwd"
awn screenshot $ID
awn close $ID
```

### TUI Application Automation

```bash
awn create htop
awn screenshot $ID --full              # see screen + UI elements
awn press $ID F2                       # open settings
awn wait $ID --text "Setup"
awn screenshot $ID --full
awn press $ID q                        # quit
awn close $ID
```

### Multi-step Pipeline

```bash
awn pipeline $ID '[
  {"type": "exec", "input": "git status"},
  {"type": "screenshot"},
  {"type": "exec", "input": "git diff --stat"},
  {"type": "screenshot"}
]' --stop-on-error
```

### Wait for Slow Operations

```bash
awn exec $ID "make build" --timeout 60000
# or with pipeline
awn pipeline $ID '[
  {"type": "type", "text": "make build"},
  {"type": "press", "keys": "Enter"},
  {"type": "wait", "stable": true, "timeout_ms": 60000},
  {"type": "screenshot"}
]'
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AWN_ADDR` | `ws://localhost:7600` | Daemon WebSocket address |
| `AWN_TOKEN` | *(none)* | Bearer token for authentication |
| `AWN_STATE_DIR` | `~/.cache/awn/sessions` | Session snapshot directory |
| `AWN_MAX_CONNECTIONS` | `10` | Max concurrent WebSocket connections |

## RPC Reference

JSON-RPC 2.0 over WebSocket at `127.0.0.1:7600`.

| Method | Params | Returns |
|--------|--------|---------|
| `ping` | none | `{status}` |
| `create` | `{command, args?, rows?, cols?}` | `{id}` |
| `screenshot` | `{id, format?, scrollback?}` | `{rows, cols, hash, lines?, history?, changes?, cursor, elements?, state?}` |
| `detect` | `{id}` | `{elements}` |
| `input` | `{id, data}` | `null` |
| `resize` | `{id, rows, cols}` | `null` |
| `mouse_click` | `{id, row, col, button?}` | `null` |
| `mouse_move` | `{id, row, col}` | `null` |
| `exec` | `{id, input, wait_text?, timeout_ms?}` | `{screen}` |
| `wait` | `{id, text?, stable?, gone?, regex?, timeout_ms?}` | `null` |
| `pipeline` | `{id, steps, stop_on_error?}` | `{results}` |
| `record` | `{id, path}` | `null` |
| `close` | `{id}` | `null` |
| `list` | none | `{sessions}` |

## Authentication

Set `AWN_TOKEN` on both daemon and client to enable Bearer token auth. Required when listening on non-loopback addresses.

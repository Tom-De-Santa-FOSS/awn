---
name: awn
description: "TUI automation for AI agents. Use when the user needs to interact with terminal applications programmatically — create sessions, take screenshots, send input, wait for text. Trigger on: 'terminal automation', 'TUI session', 'screenshot terminal', 'awn', 'awnd'."
trigger: strategy
---

# awn — TUI Automation for AI Agents

Manage headless terminal sessions via JSON-RPC 2.0 over WebSocket.

## Quick Start

```bash
awnd &                              # Start daemon (localhost:7600)
awn create bash                     # Create session
awn screenshot <id>                 # Capture terminal state
awn input <id> "ls -la\n"          # Send input
awn wait-for-text <id> "done"      # Wait for text to appear
awn close <id>                      # End session
```

## Environment Variables

- `AWN_ADDR` — Daemon address (default: `ws://localhost:7600`)
- `AWN_TOKEN` — Bearer token for authentication (optional)

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

## When to Use

- Automating interactive terminal applications (htop, vim, ncurses)
- Taking screenshots of terminal state for AI agent vision
- Sending keystrokes to running TUI programs
- Waiting for specific output before proceeding

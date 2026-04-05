<p align="center">
  <img src="awn-logo.png" alt="awn logo" width="200" />
</p>

# awn

TUI automation for AI agents. Manage headless terminal sessions — screenshot, send input, detect UI elements, and wait on terminal state.

<img src="https://skillicons.dev/icons?i=go" alt="Go" />

## Highlights

- Headless terminal sessions with full PTY emulation
- AI-friendly element detection via [awtree](https://github.com/Tom-De-Santa-FOSS/awtree)
- Agent-friendly structured detect output with refs, roles, descriptions, and tree data
- Current session tracking — create once, omit the ID from subsequent commands
- Human-friendly command aliases (`open`, `show`, `inspect`) and readable output
- JSON-RPC 2.0 over WebSocket — use from any language
- MCP server included for direct LLM tool integration
- Session persistence and restore across daemon restarts
- Named key input (Enter, Ctrl+C, arrows, function keys)
- Multi-step pipelines for batching operations
- Exec-and-wait for scripting shell interactions
- Flexible wait conditions: text match, regex, gone, stable screen

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Tom-De-Santa-FOSS/awn/master/install.sh | bash
```

## Quickstart

```bash
awn daemon start                # start the daemon
awn create yazi                 # launch a session (becomes current)
awn show                        # capture the screen (uses current session)
awn inspect                     # human-readable UI elements
awn close                       # terminate the session
```

Session IDs are tracked automatically — `create` sets the current session and subsequent commands use it. Pass `--session <id>` or a positional ID to target a specific session.

## Usage

### Current Session

`awn create` automatically sets the current session. All commands that take a session ID will use it by default:

```bash
awn create bash                        # sets current session
awn screenshot                         # uses current session
awn type "hello"                       # uses current session
awn close                              # closes and clears current session
```

Override with `--session <id>` or `-s <id>` to target a different session. The current session is stored in `~/.awn/current`.

### Command Aliases

| Alias | Command |
|-------|---------|
| `open` | `create` |
| `show` | `screenshot` |
| `inspect` | `detect` |

### Sessions

```bash
awn create <command> [args...]         # start a session (sets current)
awn list                               # show active sessions (* marks current)
awn close [id]                         # terminate session
awn ping                               # daemon health check
awn daemon start                       # start the daemon in background
awn daemon stop                        # stop the daemon
awn daemon status                      # check daemon status
```

### Screen Capture

```bash
awn screenshot [id]                    # render screen as text
awn screenshot [id] --json             # full JSON response
awn screenshot [id] --full             # screen + detected elements
awn screenshot [id] --structured       # semantic state + detected elements (JSON)
awn screenshot [id] --diff             # changed rows since last screenshot
awn screenshot [id] --scrollback 100   # include scrollback history
```

### Input

```bash
awn type [id] "hello world"            # send literal text
awn press [id] Enter                   # send a named key
awn press [id] Ctrl+C                  # send key combo
awn input [id] "raw data"              # send raw bytes
awn mouse-click [id] 10 12             # click at row col
awn mouse-move [id] 10 12              # move cursor to row col
```

### Waiting

```bash
awn wait [id] --text "Status"          # block until text appears
awn wait [id] --gone "Loading"         # block until text disappears
awn wait [id] --regex "v\d+\.\d+"     # block until regex matches
awn wait [id] --stable                 # block until screen stops changing
awn wait [id] --timeout 10000          # set timeout in ms (default 5000)
```

### Automation

```bash
awn exec [id] "ls -la"                         # run command, wait for output
awn exec [id] "make" --wait-text "done"        # wait for specific text
awn exec [id] "cargo build" --timeout 30000    # custom timeout

awn pipeline [id] '[                           # batch multiple steps
  {"type": "type", "text": "ls\r"},
  {"type": "wait", "text": "$"},
  {"type": "screenshot"}
]'
```

### Other

```bash
awn detect [id]                        # human-readable element list (default)
awn detect [id] --json                 # full structured JSON for agents
awn detect [id] --verbose              # verbose human-readable with refs and bounds
awn resize [id] 40 120                 # resize session rows/cols
awn record [id] session.cast           # write asciicast v2 recording
```

Detect defaults to human-readable output showing roles and labels. Use `--json` for the full structured payload with refs, tree data, and viewport information. Use `--verbose` for a detailed human-readable view including refs, bounds, and descriptions.

## Go SDK

Embed the driver directly:

```go
import (
    "time"
    "github.com/tom/awn"
    "github.com/tom/awn/awtreestrategy"
)

d := awn.NewDriver()
s, _ := d.Session("yazi")
s.WaitForText("Status", 5*time.Second)
elements := s.FindAll(awtreestrategy.New())
s.SendKeys("q")
d.Close(s.ID)
```

Or connect to the daemon over WebSocket:

```go
import "github.com/tom/awn/client"

c := client.New("ws://127.0.0.1:7600")
c.Ping()
session, _ := c.Create("yazi")
screen, _ := c.Screenshot(session.ID)
_ = c.Resize(session.ID, 40, 120)
elements, _ := c.Detect(session.ID)
structured, _ := c.DetectStructured(session.ID)
_ = c.Record(session.ID, "session.cast")
_ = c.Input(session.ID, "q")
sessions, _ := c.List()
_ = c.Close(session.ID)
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `AWN_ADDR` | `ws://localhost:7600` | Daemon address |
| `AWN_TOKEN` | *(none)* | Bearer token for authentication |
| `AWN_STATE_DIR` | `~/.cache/awn/sessions` | Session snapshot directory |
| `AWN_MAX_CONNECTIONS` | `10` | Max concurrent WebSocket connections |

## RPC

JSON-RPC 2.0 over WebSocket. See [docs/rpc.md](docs/rpc.md) for the full method reference.

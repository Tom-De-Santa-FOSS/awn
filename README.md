# awn

TUI automation for AI agents. Manage headless terminal sessions — screenshot, send input, detect UI elements, and wait on terminal state.

<img src="https://skillicons.dev/icons?i=go" alt="Go" />

## Highlights

- Headless terminal sessions with full PTY emulation
- AI-friendly element detection via [awtree](https://github.com/Tom-De-Santa-FOSS/awtree)
- JSON-RPC 2.0 over WebSocket — use from any language
- MCP server included for direct LLM tool integration
- Session persistence and restore across daemon restarts

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Tom-De-Santa-FOSS/awn/master/install.sh | bash
```

## Quickstart

```bash
awnd &                          # start the daemon
awn create yazi                 # launch a session
awn screenshot <id>             # capture the screen
```

## Usage

```bash
awn create <command>                    # start a session
awn screenshot <id>                     # capture screen
awn screenshot <id> --full              # screen + detected elements as JSON
awn screenshot <id> --diff --json       # changed rows since last screenshot
awn detect <id>                         # accessibility tree
awn input <id> "j"                      # send keystrokes
awn resize <id> 40 120                  # resize session rows/cols
awn mouse-click <id> 10 12             # send mouse click
awn mouse-move <id> 10 12              # send mouse move
awn wait <id> "Status"                  # block until text appears
awn record <id> session.cast            # write asciicast v2 recording
awn watch <id>                          # live session viewer
awn list                                # show active sessions
awn ping                                # health check
awn close <id>                          # terminate session
```

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
session, _ := c.Create("yazi")
screen, _ := c.Screenshot(session.ID)
_ = c.Input(session.ID, "q")
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

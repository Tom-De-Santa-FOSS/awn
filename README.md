# awn

TUI automation for AI agents. Manage headless terminal sessions — screenshot, send input, detect UI elements, and wait on terminal state.

<img src="https://skillicons.dev/icons?i=go" alt="Go" />

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Tom-De-Santa-FOSS/awn/master/install.sh | bash
```

## Usage

```bash
awnd &                  # start daemon
awn create yazi         # start a session
awn screenshot <id>     # capture screen
awn screenshot <id> --full  # include lines plus detected elements as JSON
awn screenshot <id> --diff --json  # changed rows since last screenshot
awn detect <id>         # accessibility tree
awn input <id> "j"      # send keystrokes
awn resize <id> 40 120  # resize session rows/cols
awn mouse-click <id> 10 12         # send mouse click
awn mouse-move <id> 10 12          # send mouse move
awn wait <id> "Status"  # block until text appears
awn record <id> session.cast       # write asciicast v2 recording
awn ping               # verify daemon JSON-RPC health
awn close <id>          # terminate session
awn list                # show active sessions
awn watch <id>          # watch session screen in real-time
```

## Go Library

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

## Go SDK

```go
c := client.New("ws://127.0.0.1:7600")
ping, _ := c.Ping()
session, _ := c.Create("yazi")
screen, _ := c.Screenshot(session.ID)
_ = c.Input(session.ID, "q")
_ = c.Close(session.ID)

_ = ping.Status
_ = screen.Lines
```

## RPC

JSON-RPC 2.0 over WebSocket at `127.0.0.1:7600`.

| Method | Params | Returns |
|--------|--------|---------|
| `ping` | none | `{status}` |
| `create` | `{command, args?, rows?, cols?, scrollback?}` | `{id}` |
| `screenshot` | `{id, format?, scrollback?}` | `{rows, cols, hash, lines?, history?, changes?, cursor, elements?, state?}` |
| `detect` | `{id}` | `{elements}` |
| `input` | `{id, data}` | `null` |
| `resize` | `{id, rows, cols}` | `null` |
| `mouse_click` | `{id, row, col, button?}` | `null` |
| `mouse_move` | `{id, row, col}` | `null` |
| `wait_for_text` | `{id, text, timeout_ms?}` | `null` |
| `wait_for_stable` | `{id, stable_ms?, timeout_ms?}` | `null` |
| `record` | `{id, path}` | `null` |
| `close` | `{id}` | `null` |
| `list` | none | `{sessions}` |

## Auth

Set `AWN_TOKEN` env var on both daemon and CLI for Bearer token auth.

`awnd` stores restorable session snapshots under `AWN_STATE_DIR` when set, or the default user cache directory otherwise. Use `AWN_MAX_CONNECTIONS` to override the default WebSocket connection limit of `10`.

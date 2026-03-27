# awn

TUI automation for AI agents. Manage headless terminal sessions — screenshot, send input, detect UI elements, and wait on terminal state.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Tom-De-Santa-FOSS/awn/master/install.sh | bash
```

## Usage

```bash
awnd &                  # start daemon
awn create yazi         # start a session
awn screenshot <id>     # capture screen
awn detect <id>         # accessibility tree
awn input <id> "j"      # send keystrokes
awn wait <id> "Status"  # block until text appears
awn close <id>          # terminate session
awn list                # show active sessions
```

## Go Library

```go
d := awn.NewDriver()
s, _ := d.Session("yazi")
s.WaitForText("Status", 5*time.Second)
elements := s.FindAll(awtreestrategy.New())
s.SendKeys("q")
d.Close(s.ID)
```

## RPC

JSON-RPC 2.0 over WebSocket at `127.0.0.1:7600`.

| Method | Params | Returns |
|--------|--------|---------|
| `create` | `{command, args?, rows?, cols?}` | `{id}` |
| `screenshot` | `{id}` | `{rows, cols, lines, cursor}` |
| `detect` | `{id}` | `{elements}` |
| `input` | `{id, data}` | `null` |
| `wait_for_text` | `{id, text, timeout_ms?}` | `null` |
| `wait_for_stable` | `{id, stable_ms?, timeout_ms?}` | `null` |
| `close` | `{id}` | `null` |
| `list` | none | `{sessions}` |

## Auth

Set `AWN_TOKEN` env var on both daemon and CLI for Bearer token auth.

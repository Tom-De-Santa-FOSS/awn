<p align="center">
  <img src="awn-logo.png" alt="awn logo" width="200" />
</p>

# awn

TUI automation for AI agents. Manage headless terminal sessions — screenshot, send input, detect UI elements, and wait on terminal state.

<img src="https://skillicons.dev/icons?i=go" alt="Go" />

## Install

### Homebrew (macOS & Linux)

```bash
brew install Tom-De-Santa-FOSS/tap/awn
```

### Go

```bash
go install github.com/tom/awn/cmd/awn@latest
go install github.com/tom/awn/cmd/awnd@latest
go install github.com/tom/awn/cmd/awn-mcp@latest
```

### Shell script

```bash
curl -fsSL https://raw.githubusercontent.com/Tom-De-Santa-FOSS/awn/master/install.sh | bash
```

### Binary releases

Download from [GitHub Releases](https://github.com/Tom-De-Santa-FOSS/awn/releases).

## Quickstart

```bash
awn daemon start                # start the daemon
awn create yazi                 # launch a session (becomes current)
awn show                        # capture the screen
awn inspect                     # human-readable UI elements
awn close                       # terminate the session
```

Session IDs are tracked automatically — `create` sets the current session and subsequent commands use it. Pass `--session <id>` or `-s <id>` to target a specific session.

## Architecture

```
Agent → CLI / SDK → JSON-RPC 2.0 (Unix socket or WebSocket) → Daemon → PTY + VT100 + awtree
```

The daemon manages multiple terminal sessions, each with its own pseudo-terminal, VT100 emulator, and element detector ([awtree](https://github.com/Tom-De-Santa-FOSS/awtree)). By default, communication uses a Unix domain socket at `~/.awn/daemon.sock`. TCP mode is available with `--tcp` for remote automation.

## CLI Reference

### Daemon

```bash
awn daemon start                       # start (Unix socket, default)
awn daemon stop                        # stop
awn daemon status                      # check status, shows transport type
```

### Sessions

```bash
awn create <cmd> [args...] [flags]     # start session (sets current)
    --env KEY=VALUE                    # set env var (repeatable)
    --dir /path                        # working directory
    --record                           # start recording immediately
    --record-path <path>               # custom recording path
awn list                               # show active sessions (* = current)
awn close [id]                         # terminate session
```

### Screen Capture

```bash
awn screenshot [id]                    # text lines (default)
awn screenshot [id] --json             # full JSON response
awn screenshot [id] --full             # lines + elements + state
awn screenshot [id] --structured       # elements + state (no lines)
awn screenshot [id] --diff             # changed rows since last capture
awn screenshot [id] --scrollback N     # include scrollback history
```

All responses include a `hash` field (SHA-256) for change detection.

| Format | Lines | Elements | State | Changes |
|--------|-------|----------|-------|---------|
| *(default)* | yes | no | no | no |
| `--full` | yes | yes | yes | no |
| `--structured` | no | yes | yes | no |
| `--diff` | no | no | yes | yes |

### Input

```bash
awn type [id] "text"                   # send text (no Enter)
awn press [id] <key> [--repeat N]      # send named key(s)
awn input [id] "raw"                   # send raw bytes/escape sequences
awn mouse-click [id] <row> <col> [btn] # click at position
awn mouse-move [id] <row> <col>        # move cursor
```

### Detection

```bash
awn detect [id]                        # human-readable (default)
awn detect [id] --json                 # full structured JSON
awn detect [id] --verbose              # detailed with refs and bounds
```

### Waiting

```bash
awn wait [id] --text "Status"          # until text appears
awn wait [id] --gone "Loading"         # until text disappears
awn wait [id] --regex "v\d+\.\d+"     # until regex matches
awn wait [id] --stable                 # until screen stops changing
awn wait [id] --timeout 10000          # timeout in ms (default 5000)
```

### Automation

```bash
awn exec [id] "ls -la"                 # type + Enter + wait stable
awn exec [id] "make" --wait-text "done"
awn exec [id] "cargo build" --timeout 30000

awn pipeline [id] '[
  {"type": "exec", "input": "git status"},
  {"type": "screenshot"},
  {"type": "press", "keys": "q"}
]' --stop-on-error
```

### Recording

```bash
awn create bash --record               # record from first byte
awn record [id] session.cast           # start recording post-hoc
```

### Command Aliases

| Alias | Command |
|-------|---------|
| `open` | `create` |
| `show` | `screenshot` |
| `inspect` | `detect` |

## Go SDK

```go
import "github.com/tom/awn/sdk"

c, err := sdk.Connect()                                    // Unix socket (default)
c, err := sdk.Connect(sdk.WithAddr("ws://..."), sdk.WithToken("...")) // TCP

s, err := c.Create(ctx, "bash")
scr, err := c.Exec(ctx, s.ID, "ls -la", sdk.WaitStable())
fmt.Println(scr.Lines)

result, err := c.Detect(ctx, s.ID)
for _, el := range result.Elements {
    fmt.Println(el.Role, el.Label)
}

c.Close(ctx, s.ID)
```

Full SDK documentation: [docs/sdk.md](docs/sdk.md)

## RPC Reference

JSON-RPC 2.0 over WebSocket. See [docs/rpc.md](docs/rpc.md).

## Error Catalog

All errors are structured with `code`, `category`, `message`, `retryable`, `suggestion`, and optional `context`. See [docs/errors.md](docs/errors.md).

## Security

### Default: Unix domain socket

The daemon listens on `~/.awn/daemon.sock` with `0600` permissions. Only the owning user can connect.

### Optional: TCP mode

TCP requires an explicit `--tcp` flag on the daemon and `AWN_TOKEN` must be set:

```bash
AWN_TOKEN=secret awnd --tcp
AWN_TOKEN=secret awn screenshot
```

The daemon refuses to start in TCP mode without a token.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AWN_SOCKET` | `~/.awn/daemon.sock` | Unix socket path |
| `AWN_ADDR` | *(none)* | TCP WebSocket address (enables TCP mode) |
| `AWN_TOKEN` | *(none)* | Bearer token (required for TCP) |
| `AWN_STATE_DIR` | `~/.cache/awn/sessions` | Session persistence directory |
| `AWN_MAX_CONNECTIONS` | `10` | Max concurrent WebSocket connections |

## Supported Keys

`Enter`, `Tab`, `Backspace`, `Escape`, `Space`, `Delete`, `Up`, `Down`, `Left`, `Right`, `Home`, `End`, `PageUp`, `PageDown`, `Insert`, `F1`–`F12`, `Ctrl+A`–`Ctrl+Z`, `Ctrl+[`, `Ctrl+]`, `Ctrl+\`

# Go SDK

The `sdk` package provides a typed Go client for the awn daemon.

```
go get github.com/tom/awn
```

Import:

```go
import "github.com/tom/awn/sdk"
```

## Connecting

```go
// Default: Unix socket at ~/.awn/daemon.sock
c, err := sdk.Connect()

// TCP with authentication
c, err := sdk.Connect(
    sdk.WithAddr("ws://localhost:7600"),
    sdk.WithToken("my-secret"),
)

// Custom Unix socket path
c, err := sdk.Connect(sdk.WithSocket("/tmp/awn.sock"))
```

Environment variables `AWN_ADDR`, `AWN_SOCKET`, and `AWN_TOKEN` are respected as defaults.

## Creating Sessions

```go
// Simple
s, err := c.Create(ctx, "bash")

// With full options
s, err := c.CreateWithOpts(ctx, "bash", sdk.CreateOpts{
    Rows:       40,
    Cols:       120,
    Env:        map[string]string{"TERM": "xterm-256color", "DEBUG": "1"},
    Dir:        "/tmp",
    Record:     true,
    RecordPath: "./demo.cast",
})
```

## Screenshots

```go
// Basic text capture
scr, err := c.Screenshot(ctx, s.ID)
fmt.Println(scr.Lines)    // []string
fmt.Println(scr.Hash)     // SHA-256 for change detection

// Full: lines + elements + state
scr, err := c.Screenshot(ctx, s.ID, sdk.WithFull())

// Structured: elements + state only (no lines)
scr, err := c.Screenshot(ctx, s.ID, sdk.WithStructured())

// Diff: changed rows since last capture
scr, err := c.Screenshot(ctx, s.ID, sdk.WithDiff())

// With scrollback history
scr, err := c.Screenshot(ctx, s.ID, sdk.WithScrollback(100))
```

## Input

```go
// Type text (no Enter)
c.Type(ctx, s.ID, "hello world")

// Press named keys
c.Press(ctx, s.ID, "Enter")
c.Press(ctx, s.ID, "Ctrl+C")

// Press a key multiple times
c.PressRepeat(ctx, s.ID, "Down", 5)

// Raw bytes
c.Input(ctx, s.ID, "\x1b[A") // up arrow escape sequence
```

## Exec (Type + Enter + Wait)

```go
// Wait for screen to stabilize (default)
scr, err := c.Exec(ctx, s.ID, "ls -la")

// Wait for specific text
scr, err := c.Exec(ctx, s.ID, "make build", sdk.WaitText("done"))

// Custom timeout
scr, err := c.Exec(ctx, s.ID, "cargo build", sdk.WaitStable(), sdk.WithTimeout(60000))
```

## Wait Conditions

```go
c.Wait(ctx, s.ID, sdk.WaitText("$"))          // text appears
c.Wait(ctx, s.ID, sdk.WaitGone("Loading"))     // text disappears
c.Wait(ctx, s.ID, sdk.WaitRegex(`v\d+\.\d+`)) // regex matches
c.Wait(ctx, s.ID, sdk.WaitStable())            // screen stops changing
c.Wait(ctx, s.ID, sdk.WaitText("$"), sdk.WithTimeout(10000))
```

## UI Detection

```go
result, err := c.Detect(ctx, s.ID)
for _, el := range result.Elements {
    fmt.Printf("%s %q at (%d,%d)\n", el.Role, el.Label, el.Bounds.Row, el.Bounds.Col)
}
```

## Mouse

```go
c.MouseClick(ctx, s.ID, 10, 20)        // left click at row 10, col 20
c.MouseClick(ctx, s.ID, 10, 20, 1)     // right click
c.MouseMove(ctx, s.ID, 5, 15)          // move cursor
```

## Pipeline

```go
result, err := c.Pipeline(ctx, s.ID, []sdk.Step{
    {Type: "exec", Input: "git status"},
    {Type: "screenshot"},
    {Type: "press", Keys: "q"},
}, sdk.StopOnError())

for _, r := range result.Results {
    if r.Error != "" {
        fmt.Printf("step %d failed: %s\n", r.Step, r.Error)
    }
    if r.Screen != nil {
        fmt.Println(r.Screen.Lines)
    }
}
```

## Session Management

```go
// List sessions
resp, err := c.List(ctx)
fmt.Println(resp.Sessions) // []string

// Close session
c.Close(ctx, s.ID)

// Resize
c.Resize(ctx, s.ID, 50, 200)

// Record (post-hoc)
c.Record(ctx, s.ID, "./recording.cast")

// Health check
c.Ping(ctx)

// Disconnect (no-op currently, for forward compat)
c.Disconnect()
```

## Error Handling

All errors from the SDK are structured. See [errors.md](errors.md) for the full catalog.

```go
import "github.com/tom/awn"

err := c.Screenshot(ctx, "bad-id")

// Check by code
if awn.ErrorCode(err) == "SESSION_NOT_FOUND" { ... }

// Check retryability
if awn.IsRetryable(err) { ... }

// Unwrap
var ae *awn.AwnError
if errors.As(err, &ae) {
    log.Printf("code=%s category=%s suggestion=%s", ae.Code, ae.Category, ae.Suggestion)
}
```

## Testing

Start the daemon in `TestMain`:

```go
func TestMain(m *testing.M) {
    // Start awnd in background
    cmd := exec.Command("awnd")
    cmd.Start()
    defer cmd.Process.Kill()
    
    time.Sleep(500 * time.Millisecond)
    os.Exit(m.Run())
}
```

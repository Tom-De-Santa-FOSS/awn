# Plan: MCP Server Transport + Live Watch Mode

## Goal
Ship two features that close AWN's biggest gaps vs. competitors: (1) MCP server mode so any MCP-aware agent can use AWN as a tool, and (2) `awn watch <id>` so humans can spectate AI agent terminal sessions in real-time.

## Beads
- `awn-71i` — MCP Server Transport
- `awn-gdz` — Live Watch Mode (awn watch)

## Phases
- [x] Phase 1: MCP Server Transport
- [x] Phase 2: Subscribe RPC (shared infrastructure for watch + future streaming)
- [x] Phase 3: Live Watch Mode (`awn watch`)
- [x] Phase 4: Integration testing & docs

## Current Phase
All phases complete.

---

## Phase 1: MCP Server Transport

### Design Decision: Separate binary, not a flag on awnd

**Why:** MCP servers communicate over stdio (stdin/stdout). The existing `awnd` is a WebSocket daemon. Mixing both in one binary creates complexity around which transport is active. A separate `cmd/awn-mcp/main.go` binary keeps things clean — same pattern as `cmd/awnd` and `cmd/awn`.

**Architecture:**
```
cmd/awn-mcp/main.go
  └─ Creates Driver + Handler (same as awnd)
  └─ Wraps Handler methods as MCP tools via mark3labs/mcp-go
  └─ Serves over stdio via server.ServeStdio()
```

The key insight: `rpc.Handler` already implements `Dispatcher` and has all the business logic. The MCP binary just wraps the same Handler methods as MCP tool definitions.

### MCP Tools to Expose

Each maps 1:1 to an existing RPC method:

| MCP Tool | Params | Returns | Maps to |
|----------|--------|---------|---------|
| `awn_create` | `command: string, args?: string[], rows?: int, cols?: int` | `{id: string}` | Handler.Create |
| `awn_screenshot` | `id: string, format?: string` | ScreenResponse JSON | Handler.Screenshot |
| `awn_input` | `id: string, data: string` | success message | Handler.Input |
| `awn_wait_for_text` | `id: string, text: string, timeout_ms?: int` | success message | Handler.WaitForText |
| `awn_wait_for_stable` | `id: string, stable_ms?: int, timeout_ms?: int` | success message | Handler.WaitForStable |
| `awn_detect` | `id: string` | DetectResponse JSON | Handler.Detect |
| `awn_close` | `id: string` | success message | Handler.Close |
| `awn_list` | (none) | `{sessions: string[]}` | Handler.List |

### Implementation Steps

1. `go get github.com/mark3labs/mcp-go`
2. Create `cmd/awn-mcp/main.go`:
   - Create `Driver` + `awtreestrategy.New()` + `rpc.NewHandler(driver, strategy)`
   - Define 8 MCP tools with `mcp.NewTool()`
   - For each tool handler: unmarshal params → call `handler.Dispatch(method, params)` → return `mcp.NewToolResultText(json)`
   - `server.ServeStdio(s)`
   - Handle SIGINT/SIGTERM → `driver.CloseAll()`
3. Add `awn-mcp` target to Makefile
4. Tests: tool registration + handler dispatch integration tests

### Files to Create/Modify
- **Create:** `cmd/awn-mcp/main.go` (~150 lines)
- **Modify:** `Makefile` (add build target)
- **Modify:** `go.mod` (add mcp-go dependency)

---

## Phase 2: Subscribe RPC

### Why This Phase Exists
Both `awn watch` (Phase 3) and future streaming consumers need push-based screen updates. Rather than build polling into the watch command, add a proper subscribe mechanism that the watch command consumes.

### Design

Add a `subscribe` method to the WebSocket RPC handler that sends JSON-RPC **notifications** (no `id` field) whenever the session's `updated` channel fires.

```go
// New RPC method
"subscribe" → Handler.Subscribe(SubscribeRequest)
// Sends notifications on the same WebSocket connection:
// {"jsonrpc":"2.0","method":"screen_update","params":{...ScreenResponse...}}
```

**Key details:**
- Debounce: max 30 updates/sec (33ms minimum interval between pushes)
- Uses existing `Session.updated` channel (buffered(1), non-blocking send)
- Subscribes via a new goroutine per subscription that listens on `updated`
- Unsubscribe on: explicit `unsubscribe` call, WebSocket close, or session close
- The response to `subscribe` is immediate `{subscribed: true}` — updates come as notifications

### Changes Required
- **Modify:** `internal/rpc/handler.go` — add Subscribe/Unsubscribe methods, notification callback mechanism
- **Modify:** `internal/transport/ws.go` — support sending notifications (server-initiated messages)
- **Modify:** `session.go` — add `Subscribe() <-chan struct{}` method that returns a new channel fed by readLoop (fan-out from single `updated` channel to multiple subscribers)

### Session Fan-Out Design
Current: single `updated` channel, buffered(1). Only one reader can reliably consume.

New: add `session.Subscribe() (id string, ch <-chan struct{})` and `session.Unsubscribe(id string)`:
```go
type Session struct {
    // ... existing fields ...
    subscribers   map[string]chan struct{}  // fan-out channels
    subscribersMu sync.RWMutex
}
```
readLoop signals all subscriber channels (non-blocking). Each subscriber gets its own buffered(1) channel.

---

## Phase 3: Live Watch Mode (`awn watch`)

### Design

`awn watch <session-id>` connects to `awnd` via WebSocket, subscribes to screen updates, and renders the terminal screen with ANSI formatting.

**Rendering approach:** Raw ANSI escape sequences. No TUI framework dependency.
- Clear screen: `\033[2J\033[H`
- Position cursor: `\033[row;colH`
- Set colors: `\033[38;5;Nm` (FG) / `\033[48;5;Nm` (BG)
- Set attrs: bold `\033[1m`, italic `\033[3m`, underline `\033[4m`, etc.
- Reset: `\033[0m`

**Status bar:** Bottom line shows session ID, state (idle/active/waiting), and elapsed time.

**Terminal size handling:**
- v1: require user terminal to be >= session size. If smaller, show warning.
- Render session content at its native size, ignore user terminal size mismatch.

### Implementation Steps

1. Add `watch` subcommand to `cmd/awn/main.go`
2. Create `cmd/awn/watch.go`:
   - Dial WebSocket to awnd
   - Send `subscribe` RPC with session ID
   - On each `screen_update` notification:
     - Parse ScreenResponse
     - Render cells with ANSI escapes
     - Show status bar
   - Handle Ctrl+C → send `unsubscribe` → exit
3. Handle raw terminal mode (disable line buffering for clean rendering)

### Files to Create/Modify
- **Create:** `cmd/awn/watch.go` (~200 lines)
- **Modify:** `cmd/awn/main.go` (add watch command routing)

---

## Phase 4: Integration Testing & Docs

1. E2E test: start `awn-mcp`, create session, screenshot, input, close — all via MCP stdio
2. E2E test: start `awnd`, connect watcher, verify updates arrive
3. Update README.md: MCP setup instructions (Claude Code config snippet), watch usage
4. Update Makefile: `make install` builds all three binaries

---

## Findings
(populated during implementation)

## Progress Log
(populated during implementation)

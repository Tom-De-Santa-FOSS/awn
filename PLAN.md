# Plan: awn TDD Hardening

## Goal
Fix critical security, concurrency, and quality issues across the awn codebase using TDD, with 4 parallel work units and zero file ownership conflicts.

## Phases
- [ ] Phase 1: Units A + B + C (parallel, no shared files)
- [ ] Phase 2: Unit D (depends on Unit B for exported constants, Unit C for Dispatcher interface)

---

## Unit A: Security and Transport

**Owned files:** `internal/transport/ws.go` (modify), `internal/transport/ws_test.go` (create)

**Context:** The WebSocket server in `ws.go` has three security issues: it binds to all interfaces via the addr flag default, `CheckOrigin` always returns true, and there is no authentication. It also silently ignores `json.Marshal` and `WriteMessage` errors. The server currently takes a concrete `*rpc.Handler` -- change it to accept the `Dispatcher` interface defined in Unit C's contract below. The `JSONRPCRequest`, `JSONRPCResponse`, and `JSONRPCError` types stay in this file.

**Contract -- this unit exposes:**
```go
// Dispatcher is defined in internal/rpc/handler.go by Unit C.
// This unit IMPORTS it. Do not define it here.

// NewServer now accepts rpc.Dispatcher instead of *rpc.Handler.
func NewServer(d rpc.Dispatcher, addr string, token string) *Server

// Server.ListenAndServe unchanged signature.
func (s *Server) ListenAndServe() error
```

**Depends on:** Unit C defines and exports `rpc.Dispatcher` interface:
```go
type Dispatcher interface {
    Dispatch(method string, params json.RawMessage) (any, error)
}
```

**Tasks:**
1. Change `Server.handler` field from `*rpc.Handler` to `rpc.Dispatcher`.
2. Add `token string` field to `Server`. `NewServer` accepts `token` param.
3. In `handleWS`: if `s.token != ""`, check `r.Header.Get("Authorization") == "Bearer "+s.token`. Reject with HTTP 401 if mismatch.
4. Replace `CheckOrigin` lambda: reject requests where `Origin` header is non-empty (i.e., browser requests). `CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "" }`.
5. In `handleWS` error branch (line 92): replace `err.Error()` with `"internal error"` in the RPC error message sent to clients. Log the real error server-side.
6. Fix ignored errors: `json.Marshal` on lines 101 and 112 -- if marshal fails, log and return. `conn.WriteMessage` on lines 102 and 113 -- if write fails, log and return.
7. Tests: `/health` returns 200. Auth rejection returns 401. Missing auth when token set returns 401. Valid auth succeeds upgrade. Origin with value is rejected. Error response uses generic message. Marshal/write errors logged.

**Criteria:**
1. `go test ./internal/transport/... -v` passes
2. No import of `internal/session` from this package
3. `NewServer` accepts `rpc.Dispatcher` interface, not concrete type

---

## Unit B: Session and Concurrency

**Owned files:** `internal/session/session.go` (modify), `internal/session/manager.go` (modify), `internal/session/manager_test.go` (create)

**Context:** The session package manages PTY processes. It has a close/readLoop race condition (Close sends on `done` channel, but readLoop may have already exited; close of already-closed channel panics). WaitForText/WaitForStable busy-poll with sleep. readLoop holds write lock for the entire byte-processing loop. Session IDs are truncated UUIDs. `fmt.Sprintf("TERM=xterm-256color")` has no format verbs.

**Contract -- this unit exposes:**
```go
// session.go -- add exported constants and updated field
const (
    DefaultRows = 24
    DefaultCols = 80
)

// Session gets new unexported fields:
//   once    sync.Once       // protects done channel close
//   wg      sync.WaitGroup  // tracks readLoop goroutine
//   updated chan struct{}    // buffered(1), signaled on screen change

// PTYStarter allows injecting a fake PTY for tests.
type PTYStarter interface {
    Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error)
}

// manager.go
type Manager struct {
    sessions map[string]*Session
    mu       sync.RWMutex
    pty      PTYStarter  // nil = use default pty.StartWithSize
}
func NewManager() *Manager                           // unchanged signature
func NewManagerWithPTY(p PTYStarter) *Manager        // test constructor
```

**Tasks:**
1. Export `DefaultRows=24`, `DefaultCols=80` constants. Use them in `defaults()`.
2. Add `once sync.Once`, `wg sync.WaitGroup`, `updated chan struct{}` (buffered 1) to `Session`.
3. Fix `Create`: use full `uuid.New().String()` (remove `[:8]`). Replace `fmt.Sprintf("TERM=xterm-256color")` with string literal `"TERM=xterm-256color"`. Same for COLUMNS and LINES -- keep Sprintf there since they have `%d` verbs. Use `s.wg.Add(1)` before `go sess.readLoop()`.
4. Fix `readLoop`: `defer s.wg.Done()`. Process bytes into a local `[][]rune` copy, then lock and swap: `s.mu.Lock(); s.buf = localBuf; s.mu.Unlock()`. After unlock, non-blocking send on `s.updated` (`select { case s.updated <- struct{}{}: default: }`).
5. Fix `Close`: wrap `close(s.done)` in `sess.once.Do(func(){ close(sess.done) })`. Call `sess.ptmx.Close()` first (causes readLoop to exit on read error), then `sess.wg.Wait()`, then once.Do close, then Kill process. Remove from map before cleanup.
6. Replace busy-poll in `WaitForText`: loop selecting on `sess.updated`, `time.After(timeout)`, and `sess.done`. On updated, take screenshot and check.
7. Replace busy-poll in `WaitForStable`: same pattern, track last snapshot text and stable time.
8. Add `PTYStarter` interface. `Manager` stores optional `PTYStarter`. `NewManagerWithPTY(p)` sets it. `Create` uses `m.pty.Start(...)` if non-nil, else `pty.StartWithSize(...)`.
9. Tests using a fake PTY (pipe-based): `makeBuffer` correctness, `List` empty and populated, `get` not-found error, `Create`+`Screenshot` round-trip, `Close` idempotency (no panic on double close), `WaitForText` finds injected text, `WaitForText` times out.

**Criteria:**
1. `go test ./internal/session/... -v -race` passes
2. No data race under `-race` detector
3. Double-close does not panic

---

## Unit C: Screen and RPC

**Owned files:** `internal/screen/screen.go` (modify), `internal/screen/screen_test.go` (create), `internal/rpc/handler.go` (modify), `internal/rpc/handler_test.go` (create)

**Context:** `screen.go` has dead code (`Cell`, `Grid`) and an inefficient `Text()` method. `handler.go` Dispatch is fine structurally but needs a `Dispatcher` interface extracted for Unit A to depend on. The handler tests should use a mock session manager so they do not need real PTYs.

**Contract -- this unit exposes:**
```go
// screen.go -- Snapshot after cleanup:
type Position struct {
    Row int `json:"row"`
    Col int `json:"col"`
}
type Snapshot struct {
    Rows   int      `json:"rows"`
    Cols   int      `json:"cols"`
    Lines  []string `json:"lines"`
    Cursor Position `json:"cursor"`
}
func (s *Snapshot) Text() string  // uses strings.Join

// handler.go -- new interface:
type Dispatcher interface {
    Dispatch(method string, params json.RawMessage) (any, error)
}
// Handler implements Dispatcher (existing struct, unchanged fields).
// All request/response types unchanged.
```

**Tasks:**
1. `screen.go`: Delete `Cell` type and `Grid [][]Cell` field from `Snapshot`. Fix `Text()`: replace `+=` loop with `return strings.Join(s.Lines, "\n")`.
2. `screen_test.go`: Test `Text()` with empty snapshot, single line, multiple lines. Test that `Snapshot` JSON marshals without grid field.
3. `handler.go`: Add `Dispatcher` interface (above). Verify `Handler` satisfies it (add compile-time check: `var _ Dispatcher = (*Handler)(nil)`). In `Dispatch` default case, use JSON-RPC error code `-32601` semantics: return `fmt.Errorf("method not found: %s", method)`.
4. `handler_test.go`: Create a mock `SessionManager` interface or use the real `session.Manager` with no real PTY. Simpler: test `Dispatch` with method="unknown" returns error. Test `Dispatch` with method="list" returns valid JSON. Test `Dispatch` with method="create" and invalid params returns error. Test default timeout values in `WaitForText`/`WaitForStable` request parsing (unit test the handler methods directly with a mock manager).

**Criteria:**
1. `go test ./internal/screen/... ./internal/rpc/... -v` passes
2. `Cell` type no longer exists in screen.go
3. `Dispatcher` interface is exported from `internal/rpc`

---

## Unit D: CLI and Project Hygiene

**Owned files:** `cmd/awn/main.go` (modify), `cmd/awn/main_test.go` (create), `go.mod` (modify), `.gitignore` (create), `Makefile` (modify)

**Context:** The CLI client in `cmd/awn/main.go` silently ignores `json.Unmarshal` errors (line 53 and 128). It also needs auth token support matching Unit A's Bearer token scheme. `go.mod` marks all deps as `// indirect` but they are direct imports. The CLI hardcodes rows=24, cols=80 -- use the constants from Unit B once available. `.gitignore` should exclude `bin/`. Makefile needs a `test` target.

**Depends on:** Unit B exports `session.DefaultRows`, `session.DefaultCols`. Unit A expects `Authorization: Bearer <token>` header on WS upgrade.

**Tasks:**
1. `cmd/awn/main.go` line 53: handle `json.Unmarshal` error -- `if err := json.Unmarshal([]byte(result), &snap); err != nil { fatal("parse screenshot: " + err.Error()) }`.
2. `cmd/awn/main.go` line 111: handle `json.Marshal` error -- `data, err := json.Marshal(req); if err != nil { fatal("marshal request: " + err.Error()) }`.
3. `cmd/awn/main.go` line 128: handle `json.Unmarshal` error -- `if err := json.Unmarshal(msg, &resp); err != nil { fatal("parse response: " + err.Error()) }`.
4. Auth: read `AWN_TOKEN` env var. If set, pass `http.Header{"Authorization": []string{"Bearer " + token}}` as third arg to `websocket.DefaultDialer.Dial(addr, header)`. Need to add `"net/http"` import.
5. Replace hardcoded `24`/`80` in create params with `session.DefaultRows`/`session.DefaultCols` (adds import of `github.com/tom/awn/internal/session`).
6. `go.mod`: remove `// indirect` comments from all three deps (they are direct).
7. `.gitignore`: create with contents `bin/\n`.
8. `Makefile`: add `test` target: `go test ./... -v -race`.
9. `cmd/awn/main_test.go`: test `usage()` does not panic (call it, check no error). Test that `call` with unreachable address returns error (if feasible without daemon).

**Criteria:**
1. `go build ./cmd/awn` succeeds
2. `go mod tidy` produces no diff
3. `.gitignore` exists with `bin/`
4. `make test` target exists
5. No silently ignored unmarshal errors remain in CLI

---

## File Ownership Map

| File | Owner | Action |
|------|-------|--------|
| `internal/transport/ws.go` | Unit A | modify |
| `internal/transport/ws_test.go` | Unit A | create |
| `internal/session/session.go` | Unit B | modify |
| `internal/session/manager.go` | Unit B | modify |
| `internal/session/manager_test.go` | Unit B | create |
| `internal/screen/screen.go` | Unit C | modify |
| `internal/screen/screen_test.go` | Unit C | create |
| `internal/rpc/handler.go` | Unit C | modify |
| `internal/rpc/handler_test.go` | Unit C | create |
| `cmd/awn/main.go` | Unit D | modify |
| `cmd/awn/main_test.go` | Unit D | create |
| `cmd/awnd/main.go` | Unit D | modify |
| `go.mod` | Unit D | modify |
| `.gitignore` | Unit D | create |
| `Makefile` | Unit D | modify |

Zero overlaps. `cmd/awnd/main.go` moved to Unit D since it wires everything together and must update the `NewServer` call signature.

## Testing Strategy

Each unit uses `/tdd` -- write failing tests first, then implement fixes.

- **Unit tests (per unit):** Run `go test ./<package>/... -v -race`
- **Integration (post-merge):** Run `make build && make test` to verify everything compiles together. Specifically verify:
  - `transport.NewServer` accepts `rpc.Dispatcher` (Unit A + C contract)
  - `cmd/awnd/main.go` passes token and updated bind address to `NewServer` (Unit A + D)
  - `cmd/awn/main.go` uses `session.DefaultRows` (Unit B + D)
  - No `-race` failures under concurrent session create/close

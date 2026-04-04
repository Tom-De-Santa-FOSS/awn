# Plan: Nuke Audit — awn

## Goal
Delete all dead code, remove the entire watch/subscribe subsystem, extract magic numbers into constants, collapse redundant RPC methods, and update all docs to match.

## Phases
- [x] Phase 1: Dead code deletion — remove proven-dead functions/methods/branches
- [x] Phase 2: Watch CLI removal — delete the watch CLI, subscribe RPC, and all supporting code
- [x] Phase 3: Refactoring — extract constants, collapse redundant wait methods/routes
- [x] Phase 4: Doc updates
- [x] Phase 5: Replace stdlib log with uber-go/zap + extensive logging (LDD) — update README.md, docs/rpc.md, .claude/skills/awn/SKILL.md

## Current Phase
Phase 1

## Hit List

### Phase 1: Dead Code (SAFE — zero callers confirmed by grep)

| # | Target | File | Lines | Action |
|---|--------|------|-------|--------|
| 1 | `ErrSessionNotRunning()` | errors.go:49-58 | 10 | Delete function + comment |
| 2 | `ErrTerminal()` | errors.go:60-68 | 9 | Delete function + comment |
| 3 | `WithToken()` | client/client.go:22-26 | 5 | Delete method |
| 4 | Unreachable default in `Wait()` | internal/rpc/handler.go:487-488 | 2 | Delete dead branch |
| 5 | Unused loop var `_ = i` | cmd/awn/daemon.go:68-69 | 1 | Change `for i := range 50 { _ = i` to `for range 50 {` |

### Phase 2: Watch CLI Removal (subscribe system stays — it's a valid RPC feature)

**Files to delete entirely:**
- `cmd/awn/watch.go` — watch CLI command
- `cmd/awn/watch_test.go` — watch tests
- `cmd/awn/render.go` — renderLines/renderStatusBar (only used by watch)

**Code to remove from existing files:**
- `cmd/awn/main.go:329-334` — `case "watch":` in CLI switch
- `cmd/awn/main.go:361` — remove "watch" from error help text command list

### Phase 3: Refactoring

| # | Target | Action |
|---|--------|--------|
| 1 | `500*time.Millisecond` (4 sites in handler.go) | Extract `const defaultStableThreshold = 500 * time.Millisecond` |
| 2 | `5 * time.Second` default timeout (5 sites in handler.go) | Extract `const defaultTimeout = 5 * time.Second` |
| 3 | `wait_for_text` / `wait_for_stable` redundant RPC routes + methods | Remove routes from handler, delete `WaitForText()` and `WaitForStable()` methods, delete `WaitTextRequest` and `WaitStableRequest` types. Update MCP server to route through `wait` instead. |
| 4 | MCP `awn_wait_for_text` / `awn_wait_for_stable` tools | Rewrite to dispatch through `"wait"` RPC method with appropriate params |

### Phase 4: Doc Updates

| File | Changes needed |
|------|---------------|
| README.md | Remove `awn watch` from usage, remove `AWN_TOKEN` if watch was sole consumer (check — no, token is used by daemon auth too, keep it) |
| docs/rpc.md | Remove `wait_for_text`, `wait_for_stable` rows. Remove "Subscriptions" section. |
| .claude/skills/awn/SKILL.md | Remove `awn watch` from "Other" section. Remove `wait_for_text`, `wait_for_stable`, `subscribe`, `unsubscribe` from RPC table. |

## Findings
- `awtree` is clean — no actionable findings. Only minor: `boundsContain()` lives in detect_dialog.go but is used by tree.go. Not worth a PR.
- Subscribe system is a legitimate RPC feature used by clients — keep it. Only the `watch` TUI viewer is being removed.
- `wait_for_text`/`wait_for_stable` are used by MCP tools — must update MCP to route through unified `wait` before removing.
- `render.go` (renderLines + renderStatusBar) is ONLY used by watch.go — safe to delete entirely.

## Progress Log

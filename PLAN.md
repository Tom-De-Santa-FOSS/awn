# Plan: Phase 5 Feature Parity With Competitors

## Goal
Implement all Phase 5 items in `awn`: screen hashing, mouse input, session persistence, asciicast recording, delta screenshots, configurable connection limits, and scrollback/history.

## Beads
- `awn-u4w` — [Epic] Feature Parity with Competitors
- `awn-20g` — Add screen hashing (SHA-256)
- `awn-070` — Add mouse input support
- `awn-99n` — Add session persistence
- `awn-de2` — Add asciicast v2 recording
- `awn-afh` — Add delta/diff screenshots
- `awn-5mq` — Make connection limits configurable
- `awn-qnc` — Add scrollback/history buffer

## Phases
- [x] Phase 1: Screen hashing
- [x] Phase 2: Session model foundation for history, recording, and persistence
- [x] Phase 3: RPC and transport extensions for mouse, diff screenshots, and recording
- [x] Phase 4: Daemon startup persistence and configurable limits
- [x] Phase 5: CLI coverage, tests, and docs

## Current Phase
All phases complete.

## Findings
- `internal/rpc/ScreenResponse` is the existing screenshot contract; Phase 5 features should extend it rather than add parallel response types.
- `Session` already centralizes PTY reads and update fan-out, so history and recording belong there.
- `cmd/awnd/main.go` currently builds a plain `awn.NewDriver()` with no persistence hooks or transport config.
- WebSocket connection limits are hardcoded in `internal/transport/ws.go`.
- Restored sessions can safely expose screenshots, history, detect, and recordings from persisted snapshots, even though live PTY input cannot resume after a daemon restart.
- Diff screenshots are stateful per handler/session: the first diff returns the full screen as one change set, later diffs return only changed row groups.

## Progress Log
- [Phase 1 complete] Added `hash` to screenshot responses and verified the full Go test suite stays green.
- [Phase 2 complete] Added session scrollback capture, asciicast event capture, mouse protocol writers, and persisted session snapshot loading.
- [Phase 3 complete] Exposed `mouse_click`, `mouse_move`, `record`, diff screenshot responses, and scrollback-aware screenshots through RPC.
- [Phase 4 complete] Enabled default cache-backed persistence in `awnd` and made WebSocket connection limits configurable via `AWN_MAX_CONNECTIONS`.
- [Phase 5 complete] Added CLI support for recording, mouse actions, diff/scrollback screenshots, updated README usage, and kept the full test suite green.

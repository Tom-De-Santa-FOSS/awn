# Plan: Phase 6 API And Integration Polish

## Goal
Implement all Phase 6 items in `awn`: Go SDK, graceful CLI reconnect, resize command, recording CLI/RPC polish, enriched screenshots, ping RPC, and missing daemon/watch tests.

## Beads
- `awn-k4x` — [Epic] API & Integration Polish
- `awn-lyo` — Add Go SDK client library
- `awn-l5a` — Add graceful reconnect in CLI
- `awn-i6g` — Add resize command
- `awn-de2` — Add asciicast v2 recording
- `awn-4f9` — Enrich screenshot with element tree
- `awn-4kv` — Add health check / ping RPC
- `awn-rue` — Add tests for awnd and watch.go

## Phases
- [x] Phase 1: Ping RPC and enriched screenshot contract
- [x] Phase 2: Session resize plumbing and CLI command
- [x] Phase 3: CLI reconnect behavior for watch/call paths
- [x] Phase 4: Go SDK surface for core RPC methods
- [x] Phase 5: Daemon and watch tests, final verification

## Current Phase
All phases complete.

## Findings
- `record` support already exists in `Session`, RPC, and CLI, so Phase 6 recording likely means polishing the public API rather than building it from scratch.
- `screenshot full` already merges element detection into the screenshot response; the missing work is making that easier to consume from the CLI and SDK.
- There is already an HTTP `/health` endpoint in the transport layer, but no JSON-RPC `ping` method yet.
- `Session` has no resize method, and `cmd/awn/main.go` opens a fresh WebSocket for every `call`, so reconnect work needs client-side connection handling rather than transport changes alone.
- `cmd/awnd` has no tests; `watch.go` also has no direct tests.
- `Session.Resize` originally deadlocked because `vt10x.Resize` already acquires its own lock; the extra outer lock had to be removed.
- `watch` reconnect is now driven by a small dial/read/render loop that resubscribes after socket read failures with exponential backoff.

## Progress Log
- [Phase 1 complete] Added JSON-RPC `ping` and exposed full screenshot payloads through the CLI.
- [Phase 2 complete] Added session resize plumbing, RPC routing, CLI parsing, and fixed a live resize deadlock.
- [Phase 3 complete] Reworked `watch.go` around a reconnecting subscribe loop with exponential backoff.
- [Phase 4 complete] Added a new `client` Go SDK package covering ping, create, screenshot, input, resize, record, detect, list, and close.
- [Phase 5 complete] Added direct tests for `cmd/awnd` and `watch.go`, updated README docs, and verified targeted Phase 6 packages.

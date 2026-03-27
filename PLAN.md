# Plan: Scaffold `awn` — agent-tui clone in Go

## Goal
Scaffold a working Go project at `~/Documents/portfolio/awn` that recreates agent-tui's core functionality: a daemon that manages headless TUI sessions (PTY + VT100 emulation), exposes JSON-RPC over WebSocket for AI agents to screenshot, send input, and wait on terminal state.

## Phases
- [x] Phase 1: Project structure — go module, directory layout, go.mod with deps
- [x] Phase 2: Core domain — screen buffer types, session model, snapshot serialization
- [x] Phase 3: PTY + emulation — session manager using creack/pty, spawn/close/screenshot/input
- [x] Phase 4: Transport — JSON-RPC 2.0 over WebSocket server (daemon)
- [x] Phase 5: CLI — thin client that connects to daemon and calls RPC methods
- [x] Phase 6: Build & smoke test — Makefile, verified compilation

## Findings
- Key Go libs: creack/pty v1.1.24, google/uuid v1.6.0, gorilla/websocket v1.5.3
- bubbleterm can be swapped in later for full VT100 emulation (current readLoop is simplified)
- Architecture: daemon (awnd) + CLI client (awn) + JSON-RPC/WS transport
- Both binaries compile and build cleanly

## Progress Log
- [Phase 1-6 complete] Full scaffold built and compiling. Simplified terminal parser in readLoop — swap in bubbleterm for production VT100 support.

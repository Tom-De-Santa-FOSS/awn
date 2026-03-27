# Plan: Performance Fixes for awn

## Goal
Fix all 4 open performance/correctness issues (awn-wv7, awn-280, awn-dp6, awn-61l) using TDD on branch `feat/performance-fixes`.

## Phases
- [x] Phase 1: Fix Close() shutdown ordering (awn-wv7) — kill process first, then close fd, then wait
- [x] Phase 2: Async WS dispatch (awn-280 #1) — dispatch RPCs in goroutines with write mutex
- [x] Phase 3: Lightweight ContainsText for WaitForText (awn-280 #2) — avoid full screenshot
- [x] Phase 4: Fix WaitForStable comparison (awn-280 #3) — compare Lines slice directly, not string join
- [x] Phase 5: Graceful shutdown in awnd (awn-280 #4) — close all sessions before exit
- [x] Phase 6: Bump readLoop buffer 4KB→32KB (awn-280 #5)
- [x] Phase 7: Benchmarks for readLoop and Screenshot (awn-dp6)
- [x] Phase 8: sync.Pool for read buffer (awn-61l) — strings.Builder regressed, kept rune slice

## Current Phase
DONE

## Findings
- Close() reordered: Process.Kill() → ptmx.Close() → wg.Wait() → close(done) → Process.Wait()
- Async dispatch: goroutine per request with sync.Mutex on conn writes
- ContainsText: scans cells row-by-row, short-circuits on match
- WaitForStable: linesEqual() compares slice element-by-element instead of strings.Join
- CloseAll() added to Manager, used in awnd signal handler
- readLoop buffer: 32KB via sync.Pool
- strings.Builder REGRESSED allocs (27→123) vs rune slice — Go's string([]rune) is already optimized
- Benchmark baseline: Screenshot ~16μs/27allocs, ContainsText ~15μs/25allocs, ReadLoop ~55μs/19MB/s plain

## Progress Log
- [Phase 1 complete] Reordered Close() shutdown: Kill → Close → Wait → done → Process.Wait
- [Phase 2 complete] Async WS dispatch with write mutex; test verifies fast req completes before slow req
- [Phase 3 complete] ContainsText method + WaitForText now uses it instead of full Screenshot
- [Phase 4 complete] WaitForStable uses linesEqual() for element-wise comparison
- [Phase 5 complete] CloseAll() method + awnd signal handler calls it before exit
- [Phase 6 complete] readLoop buffer bumped to 32KB
- [Phase 7 complete] 5 benchmarks: Screenshot empty/full, ContainsText, ReadLoop plain/ANSI
- [Phase 8 complete] sync.Pool for readLoop buffer; strings.Builder reverted (regressed)

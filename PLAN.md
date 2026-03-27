# Plan: Refactor awn into a public Selenium-like library

## Goal
Restructure awn from an internal daemon into a public Go package with a Selenium WebDriver-like API: Driver manages sessions, Sessions control terminal apps, pluggable Strategies detect UI elements. awtree becomes one strategy implementation. Daemon/CLI become thin consumers.

## Phases
- [x] Phase 1: Design — nail down public API types and package layout
- [x] Phase 2: Promote core — move session/screen from internal/ to root package as public API
- [x] Phase 3: Styled screen — extend Screen to capture full cell styling (FG, BG, attrs) from vt10x
- [x] Phase 4: Strategy interface — define Strategy + Element types, add FindAll/FindOne to Session
- [x] Phase 5: awtree adapter — sub-package `awn/awtree` that bridges awn.Screen → awtree.Grid → Strategy
- [ ] Phase 6: Wire daemon — rewrite cmd/awnd and cmd/awn to use public API, add `detect` RPC method
- [ ] Phase 7: Test with lazygit — end-to-end test spawning lazygit, detecting elements

## Current Phase
Phase 6 — Wire daemon to use public API

## Findings

### Package layout (target)
```
awn/
├── driver.go           # Driver (session manager, public)
├── session.go          # Session (PTY + vt10x, public)
├── screen.go           # Screen, Cell, Color, Attr types (public)
├── element.go          # Element, Strategy interface (public)
├── option.go           # functional options (public)
├── pty.go              # PTYStarter interface + realPTY (public for testing)
├── awtree/             # awtree strategy adapter (sub-package)
│   └── strategy.go     # converts awn.Screen → awtree.Grid, calls Detect()
├── internal/
│   ├── rpc/            # JSON-RPC handler (stays internal)
│   └── transport/      # WebSocket server (stays internal)
├── cmd/
│   ├── awnd/main.go    # daemon (thin)
│   └── awn/main.go     # CLI (thin)
```

### Public API sketch
```go
// Driver manages terminal sessions
type Driver struct { ... }
func NewDriver(opts ...DriverOption) *Driver
func (d *Driver) Session(command string, args ...string) (*Session, error)
func (d *Driver) SessionWithConfig(cfg Config) (*Session, error)
func (d *Driver) Close(id string) error
func (d *Driver) CloseAll()
func (d *Driver) List() []string

// Session wraps a running terminal app
type Session struct { ID string; ... }
func (s *Session) Screen() *Screen                              // styled capture
func (s *Session) Text() string                                 // plain text shortcut
func (s *Session) SendKeys(data string) error                   // input
func (s *Session) WaitForText(text string, timeout time.Duration) error
func (s *Session) WaitForStable(stable, timeout time.Duration) error
func (s *Session) FindAll(strategy Strategy) []Element          // detect elements
func (s *Session) FindOne(strategy Strategy, match MatchFunc) (Element, error)
func (s *Session) Close() error

// Screen is a styled terminal snapshot
type Screen struct {
    Rows, Cols int
    Cells      [][]Cell
    Cursor     Position
}
func (s *Screen) Text() string        // plain text
func (s *Screen) Lines() []string     // per-line text

// Cell holds one terminal character with styling
type Cell struct {
    Char  rune
    FG    Color
    BG    Color
    Attrs Attr
}

// Strategy detects UI elements from a screen
type Strategy interface {
    Detect(screen *Screen) []Element
}

// Element is a detected UI component
type Element struct {
    Type    string
    Label   string
    Bounds  Rect
    Focused bool
}

type MatchFunc func(Element) bool
func ByLabel(label string) MatchFunc
func ByType(typ string) MatchFunc
```

### vt10x → awn.Cell mapping
- vt10x Glyph: `Char rune, Mode int16, FG Color (uint32), BG Color (uint32)`
- vt10x Mode bits (internal, lowercase): reverse=1, underline=2, bold=4, gfx=8, italic=16, blink=32, wrap=64
- vt10x defaults: DefaultFG = 1<<24, DefaultBG = 1<<24+1
- awn.Attr bits: Bold=1, Faint=2, Italic=4, Underline=8, Blink=16, Reverse=32, Conceal=64, Strike=128
- Mapping: read Mode with bitmask, rebuild as awn.Attr. Map vt10x default colors → awn DefaultColor (-1).
- Note: Mode is int16 so we cast and mask. The bit positions differ so we must remap individually.

### awtree adapter mapping
- awn.Cell → awtree.Cell: same field names, need to map Attr bit positions
- awn.Attr matches awtree.Attr ordering (both start Bold=1, etc.) — can share same values
- awn.Color = awtree.Color = int32 with -1 as default
- Adapter: iterate Screen.Cells, build awtree.Grid, call awtree.Detect(), convert awtree.Element → awn.Element

### Key decisions
- awn.Cell/Attr/Color types intentionally mirror awtree's so the adapter is zero-cost
- Strategy is a simple single-method interface — no framework, just `Detect(*Screen) []Element`
- PTYStarter stays public (exported) so users can inject fakes in their own tests
- Session.FindAll takes a strategy each time (not stored) — user picks per call
- Daemon adds a `detect` RPC method that takes a strategy name param

## Progress Log

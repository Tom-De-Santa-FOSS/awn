package awn

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hinshun/vt10x"
)

// Session wraps a running terminal application.
type Session struct {
	ID            string
	cmd           *exec.Cmd
	ptmx          *os.File
	term          vt10x.Terminal
	mu            sync.RWMutex
	once          sync.Once
	wg            sync.WaitGroup
	done          chan struct{}
	updated       chan struct{}
	subscribers   map[string]chan struct{}
	subscribersMu sync.RWMutex
	cfg           Config
	history       *historyBuffer
	events        []castEvent
	snapshot      *Screen
	restored      bool
	startedAt     time.Time
	updatedAt     time.Time
	persist       func()
}

// readBufPool reuses 32KB read buffers.
var readBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 32768)
		return &buf
	},
}

// readLoop reads PTY output and feeds it to the vt10x terminal emulator.
func (s *Session) readLoop() {
	defer s.wg.Done()

	bufp := readBufPool.Get().(*[]byte)
	defer readBufPool.Put(bufp)
	buf := *bufp

	for {
		n, err := s.ptmx.Read(buf)
		if err != nil {
			return
		}

		s.mu.Lock()
		_, _ = s.term.Write(buf[:n])
		s.appendOutput(buf[:n])
		s.mu.Unlock()

		if s.persist != nil {
			s.persist()
		}
		select {
		case s.updated <- struct{}{}:
		default:
		}
		s.notifySubscribers()
	}
}

// Text returns the plain text content of the terminal screen.
func (s *Session) Text() string {
	return s.Screen().Text()
}

// ContainsText checks if text appears on screen without building a full snapshot.
func (s *Session) ContainsText(text string) bool {
	if s.restored {
		return len(text) == 0 || containsTextLines(s.Screen().Lines(), text)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.term.Lock()
	defer s.term.Unlock()

	cols, rows := s.term.Size()
	if len(text) == 0 {
		return true
	}

	needle := []rune(text)
	line := make([]rune, 0, cols)
	for row := 0; row < rows; row++ {
		line = line[:0]
		end := cols
		for end > 0 {
			ch := s.term.Cell(end-1, row).Char
			if ch != ' ' && ch != 0 {
				break
			}
			end--
		}
		for col := 0; col < end; col++ {
			line = append(line, s.term.Cell(col, row).Char)
		}
		if runesContains(line, needle) {
			return true
		}
	}
	return false
}

func containsTextLines(lines []string, text string) bool {
	if text == "" {
		return true
	}
	for _, line := range lines {
		if strings.Contains(line, text) {
			return true
		}
	}
	return false
}

// WaitForText blocks until text appears on screen or timeout elapses.
func (s *Session) WaitForText(text string, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		if s.ContainsText(text) {
			return nil
		}
		select {
		case <-s.updated:
		case <-timer.C:
			return fmt.Errorf("timeout waiting for %q after %s", text, timeout)
		case <-s.done:
			return fmt.Errorf("session closed while waiting for %q", text)
		}
	}
}

// WaitForStable blocks until the screen stops changing for the stable duration.
// Only snapshots after an actual update signal to avoid unnecessary allocations.
func (s *Session) WaitForStable(stable, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	lastText := s.Text()
	stableSince := time.Now()

	for {
		remaining := stable - time.Since(stableSince)
		if remaining <= 0 {
			return nil
		}
		stableTimer := time.NewTimer(remaining)
		select {
		case <-s.updated:
			stableTimer.Stop()
			txt := s.Text()
			if txt != lastText {
				lastText = txt
				stableSince = time.Now()
			}
		case <-stableTimer.C:
			return nil
		case <-timer.C:
			stableTimer.Stop()
			if time.Since(stableSince) >= stable {
				return nil
			}
			return fmt.Errorf("timeout waiting for stability after %s", timeout)
		case <-s.done:
			stableTimer.Stop()
			return fmt.Errorf("session closed while waiting for stability")
		}
	}
}

// runesContains reports whether needle is a sub-slice of haystack.
func runesContains(haystack, needle []rune) bool {
	nl := len(needle)
	for i := 0; i <= len(haystack)-nl; i++ {
		match := true
		for j := 0; j < nl; j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Screen returns a snapshot of the current terminal display.
func (s *Session) Screen() *Screen {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.screenLocked()
}

func (s *Session) screenLocked() *Screen {
	if s.restored {
		return cloneScreen(s.snapshot)
	}

	s.term.Lock()
	defer s.term.Unlock()

	cols, rows := s.term.Size()
	cells := make([][]Cell, rows)
	for row := 0; row < rows; row++ {
		cells[row] = make([]Cell, cols)
		for col := 0; col < cols; col++ {
			c := s.term.Cell(col, row)
			cells[row][col] = Cell{
				Char:  c.Char,
				FG:    mapColor(c.FG),
				BG:    mapColor(c.BG),
				Attrs: mapAttrs(c.Mode),
			}
		}
	}
	cur := s.term.Cursor()
	return &Screen{
		Rows:   rows,
		Cols:   cols,
		Cells:  cells,
		Cursor: Position{Row: cur.Y, Col: cur.X},
	}
}

// FindAll runs the given strategy against the current screen and returns all detected elements.
func (s *Session) FindAll(strategy Strategy) []Element {
	return strategy.Detect(s.Screen())
}

// FindOne returns the first element matching the given function, or an error if none match.
func (s *Session) FindOne(strategy Strategy, match MatchFunc) (Element, error) {
	for _, el := range s.FindAll(strategy) {
		if match(el) {
			return el, nil
		}
	}
	return Element{}, fmt.Errorf("no matching element found")
}

// vt10x Mode bit positions (from vt10x state.go attrReverse iota, unexported).
// Pinned by TestMapAttrs_matches_vt10x_terminal regression test.
const (
	vt10xModeReverse   = 1 << 0
	vt10xModeUnderline = 1 << 1
	vt10xModeBold      = 1 << 2
	// bit 3 = gfx, not mapped
	vt10xModeItalic = 1 << 4
	vt10xModeBlink  = 1 << 5
)

// mapColor converts a vt10x color to awn Color.
func mapColor(c vt10x.Color) Color {
	if c == vt10x.DefaultFG || c == vt10x.DefaultBG {
		return DefaultColor
	}
	return Color(int32(c))
}

func mapAttrs(mode int16) Attr {
	var a Attr
	if mode&vt10xModeReverse != 0 {
		a |= AttrReverse
	}
	if mode&vt10xModeUnderline != 0 {
		a |= AttrUnderline
	}
	if mode&vt10xModeBold != 0 {
		a |= AttrBold
	}
	if mode&vt10xModeItalic != 0 {
		a |= AttrItalic
	}
	if mode&vt10xModeBlink != 0 {
		a |= AttrBlink
	}
	return a
}

// Subscribe creates a new subscriber channel that fires on screen updates.
// Returns a subscriber ID and a buffered(1) channel.
func (s *Session) Subscribe() (string, <-chan struct{}) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()

	id := uuid.NewString()
	ch := make(chan struct{}, 1)
	if s.subscribers == nil {
		s.subscribers = make(map[string]chan struct{})
	}
	s.subscribers[id] = ch
	return id, ch
}

// Unsubscribe removes a subscriber by ID.
func (s *Session) Unsubscribe(id string) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()
	delete(s.subscribers, id)
}

// notifySubscribers sends a non-blocking signal to all subscriber channels.
func (s *Session) notifySubscribers() {
	s.subscribersMu.RLock()
	defer s.subscribersMu.RUnlock()
	for _, ch := range s.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// SendKeys writes input to the session's PTY.
func (s *Session) SendKeys(data string) error {
	if s.restored {
		return fmt.Errorf("session %q is restored snapshot only", s.ID)
	}
	_, err := s.ptmx.WriteString(data)
	return err
}

func (s *Session) SendMouseMove(row, col int) error {
	return s.SendKeys(fmt.Sprintf("\x1b[<35;%d;%dM", col+1, row+1))
}

func (s *Session) SendMouseClick(row, col, button int) error {
	if err := s.SendKeys(fmt.Sprintf("\x1b[<%d;%d;%dM", button, col+1, row+1)); err != nil {
		return err
	}
	return s.SendKeys(fmt.Sprintf("\x1b[<%d;%d;%dm", button, col+1, row+1))
}

// Close terminates this session.
func (s *Session) Close() error {
	if s.restored {
		s.once.Do(func() {
			close(s.done)
		})
		return nil
	}
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.ptmx.Close()
	s.wg.Wait()
	s.once.Do(func() {
		close(s.done)
	})
	if s.cmd.Process != nil {
		s.cmd.Process.Wait() //nolint:errcheck
	}
	return nil
}

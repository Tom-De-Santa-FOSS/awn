package awn

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/hinshun/vt10x"
)

// Session wraps a running terminal application.
type Session struct {
	ID      string
	cmd     *exec.Cmd
	ptmx    *os.File
	term    vt10x.Terminal
	mu      sync.RWMutex
	once    sync.Once
	wg      sync.WaitGroup
	done    chan struct{}
	updated chan struct{}
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
		s.mu.Unlock()

		select {
		case s.updated <- struct{}{}:
		default:
		}
	}
}

// Text returns the plain text content of the terminal screen.
func (s *Session) Text() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.term.Lock()
	defer s.term.Unlock()

	cols, rows := s.term.Size()
	line := make([]rune, cols)
	var lines []string
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			line[col] = s.term.Cell(col, row).Char
		}
		lines = append(lines, strings.TrimRight(string(line), " \x00"))
	}
	return strings.Join(lines, "\n")
}

// ContainsText checks if text appears on screen without building a full snapshot.
func (s *Session) ContainsText(text string) bool {
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
func (s *Session) WaitForStable(stable, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	lastText := s.Text()
	stableSince := time.Now()

	for {
		if time.Since(stableSince) >= stable {
			return nil
		}
		select {
		case <-s.updated:
		case <-timer.C:
			return fmt.Errorf("timeout waiting for stability after %s", timeout)
		case <-s.done:
			return fmt.Errorf("session closed while waiting for stability")
		}
		txt := s.Text()
		if txt != lastText {
			lastText = txt
			stableSince = time.Now()
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

// SendKeys writes input to the session's PTY.
func (s *Session) SendKeys(data string) error {
	_, err := s.ptmx.WriteString(data)
	return err
}

// Close terminates this session.
func (s *Session) Close() error {
	return s.close()
}

func (s *Session) close() error {
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

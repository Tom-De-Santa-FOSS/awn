package session

import (
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

// DefaultRows and DefaultCols are the fallback terminal dimensions.
const (
	DefaultRows = 24
	DefaultCols = 80
)

// PTYStarter abstracts PTY creation so tests can inject a fake.
type PTYStarter interface {
	Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error)
}

// Session wraps a PTY process with terminal emulation.
type Session struct {
	ID      string
	Cmd     *exec.Cmd
	ptmx    *os.File
	rows    int
	cols    int
	term    vt10x.Terminal
	mu      sync.RWMutex
	once    sync.Once      // protects done channel close
	wg      sync.WaitGroup // tracks readLoop goroutine
	done    chan struct{}
	updated chan struct{} // buffered(1), signaled on screen change
	created time.Time
}

// Config holds session creation parameters.
type Config struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Rows    int      `json:"rows,omitempty"`
	Cols    int      `json:"cols,omitempty"`
}

func (c *Config) defaults() {
	if c.Rows == 0 {
		c.Rows = DefaultRows
	}
	if c.Cols == 0 {
		c.Cols = DefaultCols
	}
}

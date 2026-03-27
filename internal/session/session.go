package session

import (
	"os"
	"os/exec"
	"sync"
	"time"
)

// Session wraps a PTY process with terminal emulation.
type Session struct {
	ID      string
	Cmd     *exec.Cmd
	ptmx    *os.File
	rows    int
	cols    int
	buf     [][]rune
	mu      sync.RWMutex
	done    chan struct{}
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
		c.Rows = 24
	}
	if c.Cols == 0 {
		c.Cols = 80
	}
}

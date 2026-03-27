package awn

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// PTYStarter abstracts PTY creation so tests can inject a fake.
type PTYStarter interface {
	Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error)
}

// realPTY is the production PTYStarter.
type realPTY struct{}

func (realPTY) Start(cmd *exec.Cmd, ws *pty.Winsize) (*os.File, error) {
	return pty.StartWithSize(cmd, ws)
}

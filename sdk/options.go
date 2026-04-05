package sdk

import (
	"os"
	"path/filepath"
)

// --- Connection options ---

type clientConfig struct {
	addr   string // TCP WebSocket URL
	socket string // Unix socket path
	token  string
}

// Option configures how the client connects to the daemon.
type Option func(*clientConfig)

// WithAddr sets the TCP WebSocket address (e.g. "ws://localhost:7600").
func WithAddr(addr string) Option {
	return func(c *clientConfig) { c.addr = addr }
}

// WithSocket sets the Unix socket path.
func WithSocket(path string) Option {
	return func(c *clientConfig) { c.socket = path }
}

// WithToken sets the authentication token for TCP connections.
func WithToken(token string) Option {
	return func(c *clientConfig) { c.token = token }
}

func defaultSocket() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".awn", "daemon.sock")
}

// --- Create options ---

// CreateOpts configures session creation.
type CreateOpts struct {
	Args       []string
	Rows       int
	Cols       int
	Env        map[string]string
	Dir        string
	Record     bool
	RecordPath string
}

// --- Screenshot options ---

type screenshotConfig struct {
	format     string
	scrollback int
}

// ScreenshotOption configures a screenshot request.
type ScreenshotOption func(*screenshotConfig)

// WithFull returns screen lines, elements, and state.
func WithFull() ScreenshotOption {
	return func(c *screenshotConfig) { c.format = "full" }
}

// WithStructured returns elements and state without lines.
func WithStructured() ScreenshotOption {
	return func(c *screenshotConfig) { c.format = "structured" }
}

// WithDiff returns only changed rows since last screenshot.
func WithDiff() ScreenshotOption {
	return func(c *screenshotConfig) { c.format = "diff" }
}

// WithScrollback includes N lines of scrollback history.
func WithScrollback(n int) ScreenshotOption {
	return func(c *screenshotConfig) { c.scrollback = n }
}

// --- Wait options ---

type waitConfig struct {
	text    string
	gone    string
	regex   string
	stable  bool
	timeout int
}

// WaitOption configures a wait or exec condition.
type WaitOption func(*waitConfig)

// WaitText waits until the given text appears on screen.
func WaitText(text string) WaitOption {
	return func(c *waitConfig) { c.text = text }
}

// WaitGone waits until the given text disappears from screen.
func WaitGone(text string) WaitOption {
	return func(c *waitConfig) { c.gone = text }
}

// WaitRegex waits until a regex pattern matches on screen.
func WaitRegex(pattern string) WaitOption {
	return func(c *waitConfig) { c.regex = pattern }
}

// WaitStable waits until the screen stops changing.
func WaitStable() WaitOption {
	return func(c *waitConfig) { c.stable = true }
}

// WithTimeout sets the timeout in milliseconds for wait operations.
func WithTimeout(ms int) WaitOption {
	return func(c *waitConfig) { c.timeout = ms }
}

// --- Pipeline ---

// Step is a single pipeline step.
type Step struct {
	Type    string `json:"type"`
	Input   string `json:"input,omitempty"`
	Keys    string `json:"keys,omitempty"`
	Text    string `json:"text,omitempty"`
	Stable  bool   `json:"stable,omitempty"`
	Gone    string `json:"gone,omitempty"`
	Regex   string `json:"regex,omitempty"`
	Timeout int    `json:"timeout_ms,omitempty"`
	Ms      int    `json:"ms,omitempty"`
}

type pipelineConfig struct {
	stopOnError bool
}

// PipelineOption configures a pipeline execution.
type PipelineOption func(*pipelineConfig)

// StopOnError causes the pipeline to stop at the first error.
func StopOnError() PipelineOption {
	return func(c *pipelineConfig) { c.stopOnError = true }
}

// --- Detect options ---

type detectConfig struct {
	format string
}

// DetectOption configures a detect request.
type DetectOption func(*detectConfig)

// DetectStructured requests the full structured detect response.
func DetectStructured() DetectOption {
	return func(c *detectConfig) { c.format = "structured" }
}

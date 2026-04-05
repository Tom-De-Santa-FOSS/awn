package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/tom/awn"
)

// Client connects to the awn daemon and provides typed methods for all operations.
type Client struct {
	cfg clientConfig
}

// Connect creates a new Client. By default it connects via Unix socket at ~/.awn/daemon.sock.
// Use WithAddr, WithSocket, or WithToken to customize.
func Connect(opts ...Option) (*Client, error) {
	cfg := clientConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	// Resolve defaults.
	if cfg.addr == "" && cfg.socket == "" {
		if envAddr := os.Getenv("AWN_ADDR"); envAddr != "" {
			cfg.addr = envAddr
		} else if envSock := os.Getenv("AWN_SOCKET"); envSock != "" {
			cfg.socket = envSock
		} else {
			cfg.socket = defaultSocket()
		}
	}
	if cfg.token == "" {
		cfg.token = os.Getenv("AWN_TOKEN")
	}
	return &Client{cfg: cfg}, nil
}

// Disconnect is a no-op for now (connections are per-call).
// Provided for forward compatibility.
func (c *Client) Disconnect() error {
	return nil
}

// Create starts a new terminal session.
func (c *Client) Create(ctx context.Context, command string, args ...string) (*Session, error) {
	params := map[string]any{
		"command": command,
		"rows":    24,
		"cols":    80,
	}
	if len(args) > 0 {
		params["args"] = args
	}
	var resp Session
	if err := c.call(ctx, "create", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateWithOpts starts a new terminal session with full configuration.
func (c *Client) CreateWithOpts(ctx context.Context, command string, opts CreateOpts) (*Session, error) {
	params := map[string]any{
		"command": command,
	}
	if len(opts.Args) > 0 {
		params["args"] = opts.Args
	}
	rows := opts.Rows
	if rows == 0 {
		rows = 24
	}
	cols := opts.Cols
	if cols == 0 {
		cols = 80
	}
	params["rows"] = rows
	params["cols"] = cols
	if len(opts.Env) > 0 {
		params["env"] = opts.Env
	}
	if opts.Dir != "" {
		params["dir"] = opts.Dir
	}
	if opts.Record {
		params["record"] = true
	}
	if opts.RecordPath != "" {
		params["record_path"] = opts.RecordPath
	}
	var resp Session
	if err := c.call(ctx, "create", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Screenshot captures the current screen.
func (c *Client) Screenshot(ctx context.Context, id string, opts ...ScreenshotOption) (*Screen, error) {
	cfg := screenshotConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	params := map[string]any{"id": id}
	if cfg.format != "" {
		params["format"] = cfg.format
	}
	if cfg.scrollback > 0 {
		params["scrollback"] = cfg.scrollback
	}
	var resp Screen
	if err := c.call(ctx, "screenshot", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Detect identifies UI elements on screen.
func (c *Client) Detect(ctx context.Context, id string, opts ...DetectOption) (*DetectResult, error) {
	cfg := detectConfig{format: "structured"}
	for _, o := range opts {
		o(&cfg)
	}
	params := map[string]any{"id": id, "format": cfg.format}
	var resp DetectResult
	if err := c.call(ctx, "detect", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Type sends text to the session (no Enter appended).
func (c *Client) Type(ctx context.Context, id string, text string) error {
	return c.call(ctx, "input", map[string]any{"id": id, "data": text}, nil)
}

// Press sends one or more named keys to the session.
func (c *Client) Press(ctx context.Context, id string, keys ...string) error {
	for _, key := range keys {
		seq, ok := awn.ResolveKey(key)
		if !ok {
			return awn.ErrInvalidKey(key)
		}
		if err := c.call(ctx, "input", map[string]any{"id": id, "data": seq}, nil); err != nil {
			return err
		}
	}
	return nil
}

// PressRepeat sends a single key N times.
func (c *Client) PressRepeat(ctx context.Context, id string, key string, n int) error {
	seq, ok := awn.ResolveKey(key)
	if !ok {
		return awn.ErrInvalidKey(key)
	}
	return c.call(ctx, "input", map[string]any{"id": id, "data": seq, "repeat": n}, nil)
}

// Input sends raw bytes to the session.
func (c *Client) Input(ctx context.Context, id string, data string) error {
	return c.call(ctx, "input", map[string]any{"id": id, "data": data}, nil)
}

// Exec types input + Enter and waits for a condition.
func (c *Client) Exec(ctx context.Context, id string, input string, opts ...WaitOption) (*Screen, error) {
	cfg := waitConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	params := map[string]any{"id": id, "input": input}
	if cfg.text != "" {
		params["wait_text"] = cfg.text
	}
	if cfg.timeout > 0 {
		params["timeout_ms"] = cfg.timeout
	}
	var resp ExecResult
	if err := c.call(ctx, "exec", params, &resp); err != nil {
		return nil, err
	}
	return resp.Screen, nil
}

// Wait blocks until a screen condition is met.
func (c *Client) Wait(ctx context.Context, id string, opts ...WaitOption) error {
	cfg := waitConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	params := map[string]any{"id": id}
	if cfg.text != "" {
		params["text"] = cfg.text
	}
	if cfg.gone != "" {
		params["gone"] = cfg.gone
	}
	if cfg.regex != "" {
		params["regex"] = cfg.regex
	}
	if cfg.stable {
		params["stable"] = true
	}
	if cfg.timeout > 0 {
		params["timeout_ms"] = cfg.timeout
	}
	return c.call(ctx, "wait", params, nil)
}

// MouseClick clicks at the given row/col position.
func (c *Client) MouseClick(ctx context.Context, id string, row, col int, button ...int) error {
	params := map[string]any{"id": id, "row": row, "col": col}
	if len(button) > 0 {
		params["button"] = button[0]
	}
	return c.call(ctx, "mouse_click", params, nil)
}

// MouseMove moves the mouse to the given row/col position.
func (c *Client) MouseMove(ctx context.Context, id string, row, col int) error {
	return c.call(ctx, "mouse_move", map[string]any{"id": id, "row": row, "col": col}, nil)
}

// Pipeline executes a batch of steps.
func (c *Client) Pipeline(ctx context.Context, id string, steps []Step, opts ...PipelineOption) (*PipelineResult, error) {
	cfg := pipelineConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	params := map[string]any{"id": id, "steps": steps}
	if cfg.stopOnError {
		params["stop_on_error"] = true
	}
	var resp PipelineResult
	if err := c.call(ctx, "pipeline", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Resize changes the terminal dimensions.
func (c *Client) Resize(ctx context.Context, id string, rows, cols int) error {
	return c.call(ctx, "resize", map[string]any{"id": id, "rows": rows, "cols": cols}, nil)
}

// Record starts asciicast recording to the given path.
func (c *Client) Record(ctx context.Context, id string, path string) error {
	return c.call(ctx, "record", map[string]any{"id": id, "path": path}, nil)
}

// List returns all active session IDs.
func (c *Client) List(ctx context.Context) (*ListResponse, error) {
	var resp ListResponse
	if err := c.call(ctx, "list", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Close terminates a session.
func (c *Client) Close(ctx context.Context, id string) error {
	return c.call(ctx, "close", map[string]any{"id": id}, nil)
}

// Ping checks daemon health.
func (c *Client) Ping(ctx context.Context) error {
	return c.call(ctx, "ping", nil, nil)
}

// call makes a single JSON-RPC call.
func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	_ = ctx // reserved for future deadline/cancellation support

	header := http.Header{}
	if c.cfg.token != "" {
		header.Set("Authorization", "Bearer "+c.cfg.token)
	}

	var conn *websocket.Conn
	var err error
	if c.cfg.socket != "" {
		dialer := websocket.Dialer{
			NetDial: func(network, addr string) (net.Conn, error) {
				return net.Dial("unix", c.cfg.socket)
			},
		}
		conn, _, err = dialer.Dial("ws://localhost/", header)
	} else {
		conn, _, err = websocket.DefaultDialer.Dial(c.cfg.addr, header)
	}
	if err != nil {
		return awn.ErrConnectionFailed(err.Error())
	}
	defer conn.Close() //nolint:errcheck

	req := map[string]any{"jsonrpc": "2.0", "method": method, "id": 1}
	if params != nil {
		req["params"] = params
	}
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("send: %w", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("recv: %w", err)
	}

	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int             `json:"code"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data,omitempty"`
		} `json:"error"`
	}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if resp.Error != nil {
		if resp.Error.Data != nil {
			var ae awn.AwnError
			if json.Unmarshal(resp.Error.Data, &ae) == nil && ae.Code != "" {
				return &ae
			}
		}
		return fmt.Errorf("%s", resp.Error.Message)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}
	return nil
}

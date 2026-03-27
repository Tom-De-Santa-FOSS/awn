package rpc

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/tom/awn"
)

// Dispatcher is the interface satisfied by Handler, for use by transport layers.
type Dispatcher interface {
	Dispatch(method string, params json.RawMessage) (any, error)
}

// Compile-time check that Handler implements Dispatcher.
var _ Dispatcher = (*Handler)(nil)

// Handler exposes session operations as RPC methods.
type Handler struct {
	driver   *awn.Driver
	strategy awn.Strategy
	routes   map[string]func(json.RawMessage) (any, error)
}

// NewHandler creates an RPC handler backed by a Driver and detection strategy.
func NewHandler(d *awn.Driver, strategy awn.Strategy) *Handler {
	h := &Handler{
		driver:   d,
		strategy: strategy,
	}
	h.routes = map[string]func(json.RawMessage) (any, error){
		"create": func(p json.RawMessage) (any, error) {
			var req CreateRequest
			if err := json.Unmarshal(p, &req); err != nil {
				return nil, errBadParams(err)
			}
			return h.Create(req)
		},
		"screenshot": func(p json.RawMessage) (any, error) {
			var req IDRequest
			if err := json.Unmarshal(p, &req); err != nil {
				return nil, errBadParams(err)
			}
			return h.Screenshot(req)
		},
		"input": func(p json.RawMessage) (any, error) {
			var req InputRequest
			if err := json.Unmarshal(p, &req); err != nil {
				return nil, errBadParams(err)
			}
			return nil, h.Input(req)
		},
		"wait_for_text": func(p json.RawMessage) (any, error) {
			var req WaitTextRequest
			if err := json.Unmarshal(p, &req); err != nil {
				return nil, errBadParams(err)
			}
			return nil, h.WaitForText(req)
		},
		"wait_for_stable": func(p json.RawMessage) (any, error) {
			var req WaitStableRequest
			if err := json.Unmarshal(p, &req); err != nil {
				return nil, errBadParams(err)
			}
			return nil, h.WaitForStable(req)
		},
		"detect": func(p json.RawMessage) (any, error) {
			var req IDRequest
			if err := json.Unmarshal(p, &req); err != nil {
				return nil, errBadParams(err)
			}
			return h.Detect(req)
		},
		"close": func(p json.RawMessage) (any, error) {
			var req IDRequest
			if err := json.Unmarshal(p, &req); err != nil {
				return nil, errBadParams(err)
			}
			return nil, h.Close(req)
		},
		"list": func(_ json.RawMessage) (any, error) {
			return h.List()
		},
	}
	return h
}

// --- Request/Response types ---

type CreateRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Rows    int      `json:"rows,omitempty"`
	Cols    int      `json:"cols,omitempty"`
}

type CreateResponse struct {
	ID string `json:"id"`
}

type IDRequest struct {
	ID string `json:"id"`
}

type InputRequest struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type WaitTextRequest struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Timeout int    `json:"timeout_ms,omitempty"`
}

type WaitStableRequest struct {
	ID      string `json:"id"`
	Stable  int    `json:"stable_ms,omitempty"`
	Timeout int    `json:"timeout_ms,omitempty"`
}

type ListResponse struct {
	Sessions []string `json:"sessions"`
}

type ScreenResponse struct {
	Rows   int         `json:"rows"`
	Cols   int         `json:"cols"`
	Lines  []string    `json:"lines"`
	Cursor awn.Position `json:"cursor"`
}

type DetectResponse struct {
	Elements []awn.Element `json:"elements"`
}

// --- Methods ---

func (h *Handler) Create(req CreateRequest) (*CreateResponse, error) {
	s, err := h.driver.SessionWithConfig(awn.Config{
		Command: req.Command,
		Args:    req.Args,
		Rows:    req.Rows,
		Cols:    req.Cols,
	})
	if err != nil {
		return nil, err
	}
	return &CreateResponse{ID: s.ID}, nil
}

func (h *Handler) Screenshot(req IDRequest) (*ScreenResponse, error) {
	sess := h.getSession(req.ID)
	if sess == nil {
		return nil, fmt.Errorf("session %q not found", req.ID)
	}

	scr := sess.Screen()
	return &ScreenResponse{
		Rows:   scr.Rows,
		Cols:   scr.Cols,
		Lines:  scr.Lines(),
		Cursor: scr.Cursor,
	}, nil
}

func (h *Handler) Input(req InputRequest) error {
	sess := h.getSession(req.ID)
	if sess == nil {
		return fmt.Errorf("session %q not found", req.ID)
	}
	return sess.SendKeys(req.Data)
}

func (h *Handler) WaitForText(req WaitTextRequest) error {
	sess := h.getSession(req.ID)
	if sess == nil {
		return fmt.Errorf("session %q not found", req.ID)
	}
	timeout := time.Duration(req.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return sess.WaitForText(req.Text, timeout)
}

func (h *Handler) WaitForStable(req WaitStableRequest) error {
	sess := h.getSession(req.ID)
	if sess == nil {
		return fmt.Errorf("session %q not found", req.ID)
	}
	stable := time.Duration(req.Stable) * time.Millisecond
	if stable == 0 {
		stable = 500 * time.Millisecond
	}
	timeout := time.Duration(req.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return sess.WaitForStable(stable, timeout)
}

func (h *Handler) Detect(req IDRequest) (*DetectResponse, error) {
	sess := h.getSession(req.ID)
	if sess == nil {
		return nil, fmt.Errorf("session %q not found", req.ID)
	}
	elements := sess.FindAll(h.strategy)
	return &DetectResponse{Elements: elements}, nil
}

func (h *Handler) Close(req IDRequest) error {
	return h.driver.Close(req.ID)
}

func (h *Handler) List() (*ListResponse, error) {
	return &ListResponse{Sessions: h.driver.List()}, nil
}

// getSession looks up a session by ID from the driver's list.
// Returns nil if not found.
func (h *Handler) getSession(id string) *awn.Session {
	return h.driver.Get(id)
}

// RPCError is a sentinel error type that carries a JSON-RPC error code.
type RPCError struct {
	Code int
	Err  error
}

func (e *RPCError) Error() string { return e.Err.Error() }
func (e *RPCError) Unwrap() error { return e.Err }

// errBadParams wraps a params decode error with code -32602.
func errBadParams(err error) *RPCError {
	return &RPCError{Code: -32602, Err: fmt.Errorf("invalid params: %w", err)}
}

// Dispatch routes a JSON-RPC method name to the appropriate handler.
func (h *Handler) Dispatch(method string, params json.RawMessage) (any, error) {
	fn, ok := h.routes[method]
	if !ok {
		return nil, &RPCError{Code: -32601, Err: fmt.Errorf("method not found: %s", method)}
	}
	result, err := fn(params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

package rpc

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/tom/awn/internal/screen"
	"github.com/tom/awn/internal/session"
)

// Handler exposes session operations as RPC methods.
type Handler struct {
	mgr *session.Manager
}

// NewHandler creates an RPC handler backed by a session manager.
func NewHandler(mgr *session.Manager) *Handler {
	return &Handler{mgr: mgr}
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
	Timeout int    `json:"timeout_ms,omitempty"` // milliseconds, default 5000
}

type WaitStableRequest struct {
	ID      string `json:"id"`
	Stable  int    `json:"stable_ms,omitempty"`  // milliseconds, default 500
	Timeout int    `json:"timeout_ms,omitempty"` // milliseconds, default 5000
}

type ListResponse struct {
	Sessions []string `json:"sessions"`
}

// --- Methods ---

func (h *Handler) Create(req CreateRequest) (*CreateResponse, error) {
	id, err := h.mgr.Create(session.Config{
		Command: req.Command,
		Args:    req.Args,
		Rows:    req.Rows,
		Cols:    req.Cols,
	})
	if err != nil {
		return nil, err
	}
	return &CreateResponse{ID: id}, nil
}

func (h *Handler) Screenshot(req IDRequest) (*screen.Snapshot, error) {
	return h.mgr.Screenshot(req.ID)
}

func (h *Handler) Input(req InputRequest) error {
	return h.mgr.Input(req.ID, req.Data)
}

func (h *Handler) WaitForText(req WaitTextRequest) error {
	timeout := time.Duration(req.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return h.mgr.WaitForText(req.ID, req.Text, timeout)
}

func (h *Handler) WaitForStable(req WaitStableRequest) error {
	stable := time.Duration(req.Stable) * time.Millisecond
	if stable == 0 {
		stable = 500 * time.Millisecond
	}
	timeout := time.Duration(req.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return h.mgr.WaitForStable(req.ID, stable, timeout)
}

func (h *Handler) Close(req IDRequest) error {
	return h.mgr.Close(req.ID)
}

func (h *Handler) List() (*ListResponse, error) {
	return &ListResponse{Sessions: h.mgr.List()}, nil
}

// Dispatch routes a JSON-RPC method name to the appropriate handler.
func (h *Handler) Dispatch(method string, params json.RawMessage) (any, error) {
	switch method {
	case "create":
		var req CreateRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		return h.Create(req)

	case "screenshot":
		var req IDRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		return h.Screenshot(req)

	case "input":
		var req InputRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		return nil, h.Input(req)

	case "wait_for_text":
		var req WaitTextRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		return nil, h.WaitForText(req)

	case "wait_for_stable":
		var req WaitStableRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		return nil, h.WaitForStable(req)

	case "close":
		var req IDRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		return nil, h.Close(req)

	case "list":
		return h.List()

	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

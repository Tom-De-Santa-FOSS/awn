package rpc

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tom/awn"
)

// NotifyFunc is a callback for sending push notifications to a client.
type NotifyFunc func(data json.RawMessage)

// SubscribeRequest is the params for the subscribe method.
type SubscribeRequest struct {
	ID string `json:"id"`
}

// subscription tracks an active screen update subscription.
type subscription struct {
	sessionID string
	subID     string
	stop      chan struct{}
}

// Dispatcher is the interface satisfied by Handler, for use by transport layers.
type Dispatcher interface {
	Dispatch(method string, params json.RawMessage) (any, error)
}

// Compile-time check that Handler implements Dispatcher.
var _ Dispatcher = (*Handler)(nil)

// Handler exposes session operations as RPC methods.
type Handler struct {
	driver        *awn.Driver
	strategy      awn.Strategy
	routes        map[string]func(json.RawMessage) (any, error)
	subscriptions map[string]*subscription // keyed by "sessionID:subID"
	subMu         sync.Mutex
	prevMu        sync.Mutex
	previousLines map[string][]string
}

// NewHandler creates an RPC handler backed by a Driver and detection strategy.
func NewHandler(d *awn.Driver, strategy awn.Strategy) *Handler {
	h := &Handler{
		driver:        d,
		strategy:      strategy,
		previousLines: make(map[string][]string),
	}
	h.routes = map[string]func(json.RawMessage) (any, error){
		"ping": func(_ json.RawMessage) (any, error) {
			return h.Ping(), nil
		},
		"create": func(p json.RawMessage) (any, error) {
			var req CreateRequest
			if err := json.Unmarshal(p, &req); err != nil {
				return nil, errBadParams(err)
			}
			return h.Create(req)
		},
		"screenshot": func(p json.RawMessage) (any, error) {
			var req ScreenshotRequest
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
		"resize": func(p json.RawMessage) (any, error) {
			var req ResizeRequest
			if err := json.Unmarshal(p, &req); err != nil {
				return nil, errBadParams(err)
			}
			return nil, h.Resize(req)
		},
		"mouse_click": func(p json.RawMessage) (any, error) {
			var req MouseRequest
			if err := json.Unmarshal(p, &req); err != nil {
				return nil, errBadParams(err)
			}
			return nil, h.MouseClick(req)
		},
		"mouse_move": func(p json.RawMessage) (any, error) {
			var req MouseRequest
			if err := json.Unmarshal(p, &req); err != nil {
				return nil, errBadParams(err)
			}
			return nil, h.MouseMove(req)
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
		"record": func(p json.RawMessage) (any, error) {
			var req RecordRequest
			if err := json.Unmarshal(p, &req); err != nil {
				return nil, errBadParams(err)
			}
			return nil, h.Record(req)
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

type PingResponse struct {
	Status string `json:"status"`
}

type IDRequest struct {
	ID string `json:"id"`
}

type InputRequest struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type ResizeRequest struct {
	ID   string `json:"id"`
	Rows int    `json:"rows"`
	Cols int    `json:"cols"`
}

type MouseRequest struct {
	ID     string `json:"id"`
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Button int    `json:"button,omitempty"`
}

type RecordRequest struct {
	ID   string `json:"id"`
	Path string `json:"path"`
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

type ScreenshotRequest struct {
	ID         string `json:"id"`
	Format     string `json:"format,omitempty"`
	Scrollback int    `json:"scrollback,omitempty"`
}

type ScreenChange struct {
	Row   int      `json:"row"`
	Lines []string `json:"lines"`
}

type ScreenResponse struct {
	Rows     int            `json:"rows"`
	Cols     int            `json:"cols"`
	Hash     string         `json:"hash"`
	BaseHash string         `json:"base_hash,omitempty"`
	Lines    []string       `json:"lines,omitempty"`
	History  []string       `json:"history,omitempty"`
	Changes  []ScreenChange `json:"changes,omitempty"`
	Cursor   awn.Position   `json:"cursor"`
	Elements []awn.Element  `json:"elements,omitempty"`
	State    string         `json:"state,omitempty"`
}

// promptSuffixes are line endings that indicate a shell/REPL waiting for input.
var promptSuffixes = []string{"$ ", "# ", "> ", "% ", ">>> ", "... ", ":"}

// inferState guesses whether the terminal is idle, active, or waiting for input.
func inferState(scr *awn.Screen) string {
	curRow := scr.Cursor.Row
	curCol := scr.Cursor.Col
	if curRow < 0 || curRow >= scr.Rows || curCol > scr.Cols {
		return "idle"
	}
	if curCol <= 0 {
		return "idle"
	}

	// Extract text before cursor on the cursor row.
	before := make([]rune, curCol)
	hasContent := false
	for c := range curCol {
		ch := scr.Cells[curRow][c].Char
		before[c] = ch
		if ch != ' ' && ch != 0 {
			hasContent = true
		}
	}
	line := string(before)

	for _, suffix := range promptSuffixes {
		if strings.HasSuffix(line, suffix) {
			return "waiting_for_input"
		}
	}

	// If cursor is positioned after non-whitespace content, something is running.
	if hasContent {
		return "active"
	}
	return "idle"
}

// buildScreenResponse constructs a ScreenResponse based on the requested format.
func buildScreenResponse(scr *awn.Screen, format string, elements []awn.Element, history []string, changes []ScreenChange, baseHash string) *ScreenResponse {
	resp := &ScreenResponse{
		Rows:     scr.Rows,
		Cols:     scr.Cols,
		Hash:     fmt.Sprintf("%x", sha256.Sum256([]byte(scr.Text()))),
		Cursor:   scr.Cursor,
		History:  history,
		Changes:  changes,
		BaseHash: baseHash,
	}
	switch format {
	case "structured":
		resp.Elements = elements
	case "full":
		resp.Lines = scr.Lines()
		resp.Elements = elements
	case "diff":
	default:
		resp.Lines = scr.Lines()
	}
	if format == "structured" || format == "full" || format == "diff" {
		resp.State = inferState(scr)
	}
	return resp
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

func (h *Handler) Ping() *PingResponse {
	return &PingResponse{Status: "ok"}
}

func (h *Handler) Screenshot(req ScreenshotRequest) (*ScreenResponse, error) {
	sess := h.getSession(req.ID)
	if sess == nil {
		return nil, fmt.Errorf("session %q not found", req.ID)
	}

	scr := sess.Screen()
	history := sess.Scrollback(req.Scrollback)
	var changes []ScreenChange
	var baseHash string

	var elements []awn.Element
	if req.Format == "structured" || req.Format == "full" {
		elements = sess.FindAll(h.strategy)
	}
	if req.Format == "diff" {
		lines := scr.Lines()
		baseHash, changes = h.diffFor(req.ID, lines)
	}

	return buildScreenResponse(scr, req.Format, elements, history, changes, baseHash), nil
}

func (h *Handler) Input(req InputRequest) error {
	sess := h.getSession(req.ID)
	if sess == nil {
		return fmt.Errorf("session %q not found", req.ID)
	}
	return sess.SendKeys(req.Data)
}

func (h *Handler) Resize(req ResizeRequest) error {
	sess := h.getSession(req.ID)
	if sess == nil {
		return fmt.Errorf("session %q not found", req.ID)
	}
	return sess.Resize(req.Rows, req.Cols)
}

func (h *Handler) MouseClick(req MouseRequest) error {
	sess := h.getSession(req.ID)
	if sess == nil {
		return fmt.Errorf("session %q not found", req.ID)
	}
	return sess.SendMouseClick(req.Row, req.Col, req.Button)
}

func (h *Handler) MouseMove(req MouseRequest) error {
	sess := h.getSession(req.ID)
	if sess == nil {
		return fmt.Errorf("session %q not found", req.ID)
	}
	return sess.SendMouseMove(req.Row, req.Col)
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

func (h *Handler) Record(req RecordRequest) error {
	sess := h.getSession(req.ID)
	if sess == nil {
		return fmt.Errorf("session %q not found", req.ID)
	}
	return sess.RecordAsciicast(req.Path)
}

func (h *Handler) List() (*ListResponse, error) {
	return &ListResponse{Sessions: h.driver.List()}, nil
}

// SubscribeSession starts a goroutine that watches for screen updates on the given session
// and calls notify with a JSON-encoded ScreenResponse for each update.
// Returns the subscriber ID for later Unsubscribe calls.
func (h *Handler) SubscribeSession(req SubscribeRequest, notify NotifyFunc) (string, error) {
	sess := h.getSession(req.ID)
	if sess == nil {
		return "", fmt.Errorf("session %q not found", req.ID)
	}

	subID, ch := sess.Subscribe()
	stop := make(chan struct{})

	sub := &subscription{
		sessionID: req.ID,
		subID:     subID,
		stop:      stop,
	}

	h.subMu.Lock()
	if h.subscriptions == nil {
		h.subscriptions = make(map[string]*subscription)
	}
	h.subscriptions[req.ID+":"+subID] = sub
	h.subMu.Unlock()

	go func() {
		defer sess.Unsubscribe(subID)
		for {
			select {
			case <-ch:
				scr := sess.Screen()
				resp := buildScreenResponse(scr, "", nil, nil, nil, "")
				data, err := json.Marshal(resp)
				if err != nil {
					continue
				}
				notify(data)
			case <-stop:
				return
			}
		}
	}()

	return subID, nil
}

// Subscribe implements the transport.Subscriber interface.
func (h *Handler) Subscribe(sessionID string, notify func(json.RawMessage)) (string, error) {
	return h.SubscribeSession(SubscribeRequest{ID: sessionID}, NotifyFunc(notify))
}

// Unsubscribe stops a subscription by session and subscriber ID.
func (h *Handler) Unsubscribe(sessionID, subID string) {
	key := sessionID + ":" + subID
	h.subMu.Lock()
	sub, ok := h.subscriptions[key]
	if ok {
		delete(h.subscriptions, key)
	}
	h.subMu.Unlock()

	if ok {
		close(sub.stop)
	}
}

func (h *Handler) diffFor(sessionID string, current []string) (string, []ScreenChange) {
	h.prevMu.Lock()
	defer h.prevMu.Unlock()
	previous := append([]string(nil), h.previousLines[sessionID]...)
	h.previousLines[sessionID] = append([]string(nil), current...)
	if len(previous) == 0 {
		return "", []ScreenChange{{Row: 0, Lines: append([]string(nil), current...)}}
	}
	baseHash := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(previous, "\n"))))
	var changes []ScreenChange
	for row := 0; row < len(current); {
		prev := ""
		if row < len(previous) {
			prev = previous[row]
		}
		if prev == current[row] {
			row++
			continue
		}
		start := row
		var lines []string
		for row < len(current) {
			prev = ""
			if row < len(previous) {
				prev = previous[row]
			}
			if prev == current[row] {
				break
			}
			lines = append(lines, current[row])
			row++
		}
		changes = append(changes, ScreenChange{Row: start, Lines: lines})
	}
	return baseHash, changes
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

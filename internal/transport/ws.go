package transport

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/tom/awn/internal/rpc"
)

const maxConcurrentDispatches = 64

// maxConcurrentConnections caps total simultaneous WebSocket connections.
const maxConcurrentConnections = 10

// Dispatcher routes JSON-RPC method calls to their handlers.
type Dispatcher interface {
	Dispatch(method string, params json.RawMessage) (any, error)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "" },
}

// JSONRPCRequest is a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      any             `json:"id"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
	ID      any           `json:"id"`
}

// JSONRPCError is a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server serves JSON-RPC 2.0 over WebSocket.
type Server struct {
	handler    Dispatcher
	addr       string
	token      string
	activeConn atomic.Int32
}

// NewServer creates a WebSocket JSON-RPC server.
func NewServer(d Dispatcher, addr string, token string) *Server {
	return &Server{handler: d, addr: addr, token: token}
}

// ListenAndServe starts the WebSocket server.
// It refuses to start without a token when the listen address is non-loopback.
func (s *Server) ListenAndServe() error {
	if s.token == "" {
		host, _, err := net.SplitHostPort(s.addr)
		if err == nil && host != "" && host != "127.0.0.1" && host != "::1" && host != "localhost" {
			return errors.New("AWN_TOKEN is required when listening on a non-loopback address")
		}
		if err == nil && host == "" {
			// Empty host means 0.0.0.0 — refuse without token.
			return errors.New("AWN_TOKEN is required when listening on all interfaces")
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWS)
	mux.HandleFunc("/health", s.handleHealth)

	log.Printf("awn daemon listening on %s", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if s.token != "" {
		got := []byte(r.Header.Get("Authorization"))
		want := []byte("Bearer " + s.token)
		if subtle.ConstantTimeCompare(got, want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	if s.token != "" {
		got := []byte(r.Header.Get("Authorization"))
		want := []byte("Bearer " + s.token)
		if subtle.ConstantTimeCompare(got, want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	if s.activeConn.Load() >= maxConcurrentConnections {
		http.Error(w, "too many connections", http.StatusServiceUnavailable)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	s.activeConn.Add(1)
	defer s.activeConn.Add(-1)
	defer conn.Close() //nolint:errcheck

	log.Printf("client connected: %s", conn.RemoteAddr())

	var wmu sync.Mutex // protects conn.WriteMessage
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentDispatches) // cap in-flight dispatches per connection

	defer wg.Wait() // wait for in-flight handlers before conn.Close

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("read: %v", err)
			return
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			wmu.Lock()
			s.sendError(conn, nil, -32700, "parse error")
			wmu.Unlock()
			continue
		}

		if req.JSONRPC != "2.0" {
			wmu.Lock()
			s.sendError(conn, req.ID, -32600, "invalid request: must be jsonrpc 2.0")
			wmu.Unlock()
			continue
		}

		sem <- struct{}{} // backpressure: block if maxConcurrentDispatches handlers in-flight
		wg.Add(1)
		go func(req JSONRPCRequest) {
			defer wg.Done()
			defer func() { <-sem }()

			result, err := s.handler.Dispatch(req.Method, req.Params)

			wmu.Lock()
			defer wmu.Unlock()

			if err != nil {
				log.Printf("dispatch %s: %v", req.Method, err)
				code := -32603
				msg := "internal error"
				var rpcErr *rpc.RPCError
				if errors.As(err, &rpcErr) {
					code = rpcErr.Code
					msg = rpcErr.Err.Error()
				}
				s.sendError(conn, req.ID, code, msg)
				return
			}

			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				Result:  result,
				ID:      req.ID,
			}
			data, err := json.Marshal(resp)
			if err != nil {
				log.Printf("marshal response: %v", err)
				s.sendError(conn, req.ID, -32603, "internal error")
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("write response: %v", err)
			}
		}(req)
	}
}

func (s *Server) sendError(conn *websocket.Conn, id any, code int, msg string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &JSONRPCError{Code: code, Message: msg},
		ID:      id,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("marshal error response: %v", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("write error response: %v", err)
	}
}

package transport

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/tom/awn"
	"github.com/tom/awn/internal/rpc"
)

const maxConcurrentDispatches = 64

// maxConcurrentConnections caps total simultaneous WebSocket connections.
const maxConcurrentConnections = 10

// Dispatcher routes JSON-RPC method calls to their handlers.
type Dispatcher interface {
	Dispatch(method string, params json.RawMessage) (any, error)
}

// Subscriber is an optional interface for dispatchers that support subscribe/unsubscribe.
type Subscriber interface {
	Subscribe(sessionID string, notify func(json.RawMessage)) (subID string, err error)
	Unsubscribe(sessionID, subID string)
}

// JSONRPCNotification is a server-initiated JSON-RPC 2.0 notification (no id).
type JSONRPCNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// subscribeRequest is used to parse subscribe/unsubscribe params.
type subscribeRequest struct {
	ID    string `json:"id"`
	SubID string `json:"sub_id,omitempty"`
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
	Data    any    `json:"data,omitempty"`
}

// Server serves JSON-RPC 2.0 over WebSocket.
type Server struct {
	handler    Dispatcher
	addr       string
	token      string
	maxConn    int32
	activeConn atomic.Int32
}

// NewServer creates a WebSocket JSON-RPC server.
func NewServer(d Dispatcher, addr string, token string) *Server {
	return &Server{handler: d, addr: addr, token: token, maxConn: maxConnectionsFromEnv()}
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

	if s.activeConn.Load() >= s.maxConn {
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

	// Track subscriptions for cleanup on disconnect.
	var activeSubs []activeSub
	var subsMu sync.Mutex

	defer func() {
		wg.Wait()
		// Clean up subscriptions on disconnect.
		if sub, ok := s.handler.(Subscriber); ok {
			subsMu.Lock()
			for _, as := range activeSubs {
				sub.Unsubscribe(as.sessionID, as.subID)
			}
			subsMu.Unlock()
		}
	}()

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
		started := make(chan struct{})
		go func(req JSONRPCRequest) {
			defer wg.Done()
			defer func() { <-sem }()
			close(started)

			// Handle subscribe/unsubscribe if the handler supports it.
			if sub, ok := s.handler.(Subscriber); ok && (req.Method == "subscribe" || req.Method == "unsubscribe") {
				s.handleSubscription(conn, &wmu, &subsMu, &activeSubs, sub, req)
				return
			}

			result, err := s.handler.Dispatch(req.Method, req.Params)

			wmu.Lock()
			defer wmu.Unlock()

			if err != nil {
				log.Printf("dispatch %s: %v", req.Method, err)
				code := -32603
				msg := "internal error"
				var errData any
				var rpcErr *rpc.RPCError
				var awnErr *awn.AwnError
				if errors.As(err, &rpcErr) {
					code = rpcErr.Code
					msg = rpcErr.Err.Error()
				} else if errors.As(err, &awnErr) {
					code = -32000 // application error
					msg = awnErr.Message
					errData = awnErr
				}
				s.sendErrorWithData(conn, req.ID, code, msg, errData)
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
		<-started
	}
}

func maxConnectionsFromEnv() int32 {
	value := os.Getenv("AWN_MAX_CONNECTIONS")
	if value == "" {
		return maxConcurrentConnections
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return maxConcurrentConnections
	}
	return int32(parsed)
}

type activeSub struct {
	sessionID string
	subID     string
}

func (s *Server) handleSubscription(conn *websocket.Conn, wmu, subsMu *sync.Mutex, activeSubs *[]activeSub, sub Subscriber, req JSONRPCRequest) {
	var params subscribeRequest
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			wmu.Lock()
			s.sendError(conn, req.ID, -32602, "invalid params")
			wmu.Unlock()
			return
		}
	}

	switch req.Method {
	case "subscribe":
		notify := func(data json.RawMessage) {
			notif := JSONRPCNotification{
				JSONRPC: "2.0",
				Method:  "screen_update",
				Params:  json.RawMessage(data),
			}
			msg, err := json.Marshal(notif)
			if err != nil {
				return
			}
			wmu.Lock()
			_ = conn.WriteMessage(websocket.TextMessage, msg)
			wmu.Unlock()
		}

		subID, err := sub.Subscribe(params.ID, notify)
		if err != nil {
			wmu.Lock()
			s.sendError(conn, req.ID, -32603, err.Error())
			wmu.Unlock()
			return
		}

		subsMu.Lock()
		*activeSubs = append(*activeSubs, activeSub{sessionID: params.ID, subID: subID})
		subsMu.Unlock()

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  map[string]any{"subscribed": true, "sub_id": subID},
			ID:      req.ID,
		}
		data, _ := json.Marshal(resp)
		wmu.Lock()
		_ = conn.WriteMessage(websocket.TextMessage, data)
		wmu.Unlock()

	case "unsubscribe":
		sub.Unsubscribe(params.ID, params.SubID)

		subsMu.Lock()
		for i, as := range *activeSubs {
			if as.sessionID == params.ID && as.subID == params.SubID {
				*activeSubs = append((*activeSubs)[:i], (*activeSubs)[i+1:]...)
				break
			}
		}
		subsMu.Unlock()

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  map[string]any{"unsubscribed": true},
			ID:      req.ID,
		}
		data, _ := json.Marshal(resp)
		wmu.Lock()
		_ = conn.WriteMessage(websocket.TextMessage, data)
		wmu.Unlock()
	}
}

func (s *Server) sendError(conn *websocket.Conn, id any, code int, msg string) {
	s.sendErrorWithData(conn, id, code, msg, nil)
}

func (s *Server) sendErrorWithData(conn *websocket.Conn, id any, code int, msg string, errData any) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &JSONRPCError{Code: code, Message: msg, Data: errData},
		ID:      id,
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		log.Printf("marshal error response: %v", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		log.Printf("write error response: %v", err)
	}
}

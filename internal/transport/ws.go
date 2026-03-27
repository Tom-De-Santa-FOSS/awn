package transport

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

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
	handler Dispatcher
	addr    string
	token   string
}

// NewServer creates a WebSocket JSON-RPC server.
func NewServer(d Dispatcher, addr string, token string) *Server {
	return &Server{handler: d, addr: addr, token: token}
}

// ListenAndServe starts the WebSocket server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWS)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("awn daemon listening on %s", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	if s.token != "" {
		if r.Header.Get("Authorization") != "Bearer "+s.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("client connected: %s", conn.RemoteAddr())

	var wmu sync.Mutex // protects conn.WriteMessage
	var wg sync.WaitGroup
	sem := make(chan struct{}, 64) // cap in-flight dispatches per connection

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

		sem <- struct{}{} // backpressure: block if 64 handlers in-flight
		wg.Add(1)
		go func(req JSONRPCRequest) {
			defer wg.Done()
			defer func() { <-sem }()

			result, err := s.handler.Dispatch(req.Method, req.Params)

			wmu.Lock()
			defer wmu.Unlock()

			if err != nil {
				log.Printf("dispatch %s: %v", req.Method, err)
				s.sendError(conn, req.ID, -32603, "internal error")
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

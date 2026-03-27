package transport

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/tom/awn/internal/rpc"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
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
	handler *rpc.Handler
	addr    string
}

// NewServer creates a WebSocket JSON-RPC server.
func NewServer(handler *rpc.Handler, addr string) *Server {
	return &Server{handler: handler, addr: addr}
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
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("client connected: %s", conn.RemoteAddr())

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("read: %v", err)
			return
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			s.sendError(conn, nil, -32700, "parse error")
			continue
		}

		if req.JSONRPC != "2.0" {
			s.sendError(conn, req.ID, -32600, "invalid request: must be jsonrpc 2.0")
			continue
		}

		result, err := s.handler.Dispatch(req.Method, req.Params)
		if err != nil {
			s.sendError(conn, req.ID, -32603, err.Error())
			continue
		}

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  result,
			ID:      req.ID,
		}
		data, _ := json.Marshal(resp)
		conn.WriteMessage(websocket.TextMessage, data)
	}
}

func (s *Server) sendError(conn *websocket.Conn, id any, code int, msg string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &JSONRPCError{Code: code, Message: msg},
		ID:      id,
	}
	data, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, data)
}

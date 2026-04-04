package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/tom/awn"
	"github.com/tom/awn/internal/rpc"
)

type Client struct {
	addr  string
	token string
}

func New(addr string) *Client {
	return &Client{addr: addr}
}

func (c *Client) Ping() (*rpc.PingResponse, error) {
	var resp rpc.PingResponse
	if err := c.call("ping", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type DetectResponse struct {
	Elements []awn.Element `json:"elements"`
}

func (c *Client) Create(command string, args ...string) (*rpc.CreateResponse, error) {
	var resp rpc.CreateResponse
	params := rpc.CreateRequest{Command: command, Args: args, Rows: awn.DefaultRows, Cols: awn.DefaultCols}
	if err := c.call("create", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Screenshot(id string) (*rpc.ScreenResponse, error) {
	var resp rpc.ScreenResponse
	if err := c.call("screenshot", rpc.ScreenshotRequest{ID: id}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Input(id, data string) error {
	return c.call("input", rpc.InputRequest{ID: id, Data: data}, nil)
}

func (c *Client) Resize(id string, rows, cols int) error {
	return c.call("resize", rpc.ResizeRequest{ID: id, Rows: rows, Cols: cols}, nil)
}

func (c *Client) Record(id, path string) error {
	return c.call("record", rpc.RecordRequest{ID: id, Path: path}, nil)
}

func (c *Client) Detect(id string) (*DetectResponse, error) {
	var resp DetectResponse
	if err := c.call("detect", rpc.IDRequest{ID: id}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) List() (*rpc.ListResponse, error) {
	var resp rpc.ListResponse
	if err := c.call("list", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Close(id string) error {
	return c.call("close", rpc.IDRequest{ID: id}, nil)
}

func (c *Client) call(method string, params any, out any) error {
	header := http.Header{}
	if c.token != "" {
		header.Set("Authorization", "Bearer "+c.token)
	}
	conn, _, err := websocket.DefaultDialer.Dial(c.addr, header)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
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
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if resp.Error != nil {
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

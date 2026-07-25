package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Minimal Chrome DevTools Protocol client over the Node inspector WebSocket.

type cdpClient struct {
	ws      *websocket.Conn
	mu      sync.Mutex
	nextID  int
	pending map[int]chan json.RawMessage

	events chan cdpEvent
}

type cdpEvent struct {
	Method string
	Params json.RawMessage
}

func inspectorWebSocketURL(port int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/json/list", port)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var targets []struct {
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
			}
			if json.Unmarshal(body, &targets) == nil && len(targets) > 0 && targets[0].WebSocketDebuggerURL != "" {
				return targets[0].WebSocketDebuggerURL, nil
			}
		}
		time.Sleep(80 * time.Millisecond)
	}
	return "", fmt.Errorf("inspector endpoint did not appear within %s", timeout)
}

func dialCDP(wsURL string) (*cdpClient, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		// The inspector can send large frames (getScriptSource ~1.8MB).
		ReadBufferSize:  1 << 20,
		WriteBufferSize: 1 << 20,
	}
	ws, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}
	ws.SetReadLimit(64 << 20)
	c := &cdpClient{
		ws:      ws,
		pending: make(map[int]chan json.RawMessage),
		events:  make(chan cdpEvent, 256),
	}
	go c.readLoop()
	return c, nil
}

func (c *cdpClient) readLoop() {
	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			c.mu.Lock()
			for _, ch := range c.pending {
				close(ch)
			}
			c.pending = map[int]chan json.RawMessage{}
			c.mu.Unlock()
			close(c.events)
			return
		}
		var msg struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		if msg.ID != 0 {
			c.mu.Lock()
			ch := c.pending[msg.ID]
			delete(c.pending, msg.ID)
			c.mu.Unlock()
			if ch != nil {
				if msg.Error != nil {
					ch <- msg.Error
				} else {
					ch <- msg.Result
				}
				close(ch)
			}
			continue
		}
		if msg.Method != "" {
			select {
			case c.events <- cdpEvent{Method: msg.Method, Params: msg.Params}:
			default: // drop if the consumer is slow; we only care about a few events
			}
		}
	}
}

// send issues a command and waits for its result (or error) with a timeout.
func (c *cdpClient) send(method string, params map[string]any, timeout time.Duration) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	payload := map[string]any{"id": id, "method": method}
	if params != nil {
		payload["params"] = params
	}
	data, _ := json.Marshal(payload)

	c.mu.Lock()
	err := c.ws.WriteMessage(websocket.TextMessage, data)
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	select {
	case res, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("%s: connection closed", method)
		}
		return res, nil
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("%s: timed out", method)
	}
}

func (c *cdpClient) close() {
	c.ws.Close()
}

// evaluate runs an expression in the default (main-process) context and returns
// the string value, if any.
func (c *cdpClient) evaluate(expr string, timeout time.Duration) (string, error) {
	res, err := c.send("Runtime.evaluate", map[string]any{
		"expression":    expr,
		"returnByValue": true,
	}, timeout)
	if err != nil {
		return "", err
	}
	var out struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	}
	if json.Unmarshal(res, &out) != nil {
		return "", nil
	}
	var s string
	if json.Unmarshal(out.Result.Value, &s) == nil {
		return s, nil
	}
	return strings.TrimSpace(string(out.Result.Value)), nil
}

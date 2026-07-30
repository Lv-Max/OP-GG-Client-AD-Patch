package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestInspectorWebSocketURL(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/json/list" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[{"webSocketDebuggerUrl":"ws://127.0.0.1:32123/session"}]`)
		}),
	}
	go server.Serve(listener)
	t.Cleanup(func() {
		server.Close()
	})

	port := listener.Addr().(*net.TCPAddr).Port
	got, err := inspectorWebSocketURL(port, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if want := "ws://127.0.0.1:32123/session"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestInspectorWebSocketURLTimesOut(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	start := time.Now()
	if _, err := inspectorWebSocketURL(port, 120*time.Millisecond); err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("timeout took too long: %s", elapsed)
	}
}

func TestSendReturnsCDPProtocolError(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()

		var request struct {
			ID int `json:"id"`
		}
		if err := connection.ReadJSON(&request); err != nil {
			t.Error(err)
			return
		}
		if err := connection.WriteJSON(map[string]any{
			"id": request.ID,
			"error": map[string]any{
				"code":    -32601,
				"message": "Method not found",
			},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	client, err := dialCDP("ws" + strings.TrimPrefix(server.URL, "http"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.close()

	_, err = client.send("Missing.method", nil, time.Second)
	if err == nil {
		t.Fatal("expected protocol error")
	}
	if message := err.Error(); !strings.Contains(message, "protocol error") ||
		!strings.Contains(message, "-32601") {
		t.Fatalf("unexpected error: %s", message)
	}
}

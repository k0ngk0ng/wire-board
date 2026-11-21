package cs16_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestFixtureEndpointKeepsHTTPAndWebSocketSourceIdentity(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/ws" {
			connection, err := websocket.Accept(writer, request, nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer connection.CloseNow()
			ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
			defer cancel()
			if err = connection.Write(ctx, websocket.MessageText, encode(map[string]any{"source": request.RemoteAddr})); err != nil {
				t.Error(err)
			}
			return
		}
		if err := json.NewEncoder(writer).Encode(map[string]any{"source": request.RemoteAddr}); err != nil {
			t.Error(err)
		}
	})
	sources := []map[string]any{}
	for _, source := range []string{"192.0.2.1", "192.0.2.41"} {
		endpoint := newSourceEndpoint(t, handler, source)
		request, err := http.NewRequest("GET", endpoint.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("X-Forwarded-For", "198.51.100.99")
		request.Header.Set("Forwarded", "for=198.51.100.99")
		response, err := endpoint.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		var httpValue map[string]string
		err = json.NewDecoder(response.Body).Decode(&httpValue)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK || !strings.HasPrefix(httpValue["source"], source+":") {
			t.Fatal("HTTP did not retain fixed test source", source, httpValue, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		header := http.Header{"X-Forwarded-For": []string{"198.51.100.99"}, "Forwarded": []string{"for=198.51.100.99"}}
		connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(endpoint.URL, "http")+"/ws", &websocket.DialOptions{HTTPClient: endpoint.Client(), HTTPHeader: header})
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		_, raw, err := connection.Read(ctx)
		connection.CloseNow()
		cancel()
		var wsValue map[string]string
		if err != nil || json.Unmarshal(raw, &wsValue) != nil || wsValue["source"] != httpValue["source"] {
			t.Fatal("WebSocket changed simulated client source", source, string(raw), err)
		}
		sources = append(sources, map[string]any{"source": source, "httpRemoteAddr": httpValue["source"], "webSocketRemoteAddr": wsValue["source"], "forwardedHeadersIgnored": true})
	}
	artifact(t, map[string]any{"passed": true, "scope": "test-only fixed RemoteAddr wrappers exercised through real HTTP and WebSocket sockets; production Handler and trust configuration unchanged", "sources": sources})
}

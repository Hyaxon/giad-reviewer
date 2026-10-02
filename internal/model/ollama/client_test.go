package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hyaxon/agentic-review/internal/model"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestChatAndUnload(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if r.URL.Path == "/api/generate" {
			if body["keep_alive"] != float64(0) {
				t.Error("unload must use keep_alive zero")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"done":true}`))}, nil
		}
		if body["stream"] != false || body["keep_alive"] != "10m" {
			t.Error("wrong session options")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"done":true,"message":{"role":"assistant","tool_calls":[{"function":{"name":"read_file","arguments":{"path":"a.go","start":1,"end":0}}}]}}`))}, nil
	})
	client, err := New("http://127.0.0.1:11434")
	if err != nil {
		t.Fatal(err)
	}
	client.http.Transport = transport
	response, err := client.Chat(context.Background(), "test", []model.Message{{Role: "user", Content: "Review"}}, nil)
	if err != nil || len(response.ToolCalls) != 1 {
		t.Fatalf("response=%+v err=%v", response, err)
	}
	if err := client.Unload(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
}

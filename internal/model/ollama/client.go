package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hyaxon/agentic-review/internal/model"
)

// Client uses the native Ollama API. No GitHub credentials are sent to it.
type Client struct {
	endpoint string
	http     *http.Client
}

func New(endpoint string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("Ollama endpoint must be an http(s) origin")
	}
	return &Client{endpoint: strings.TrimSuffix(endpoint, "/"), http: &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) post(ctx context.Context, path string, body any) ([]byte, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+path, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("Ollama request canceled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("Ollama request failed (is Ollama running?): %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned HTTP %d; check the model is downloaded and supports tools", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1024*1024 {
		return nil, errors.New("Ollama response exceeds 1 MiB")
	}
	return data, nil
}
func (c *Client) Chat(ctx context.Context, tag string, messages []model.Message, tools []model.Tool) (model.Message, error) {
	data, err := c.post(ctx, "/api/chat", map[string]any{"model": tag, "messages": messages, "tools": tools, "stream": false, "keep_alive": "10m", "options": map[string]any{"num_ctx": 32768, "num_predict": 4096, "temperature": 0.1}})
	if err != nil {
		return model.Message{}, err
	}
	var response struct {
		Message      model.Message `json:"message"`
		Done         bool          `json:"done"`
		DoneReason   string        `json:"done_reason"`
		Error        string        `json:"error"`
		PromptTokens int           `json:"prompt_eval_count"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return model.Message{}, fmt.Errorf("decode Ollama response: %w", err)
	}
	if response.Error != "" || !response.Done || response.Message.Role != "assistant" {
		return model.Message{}, errors.New("invalid or incomplete Ollama response")
	}
	if response.DoneReason == "length" {
		return model.Message{}, errors.New("Ollama generation limit reached; review incomplete")
	}
	if response.PromptTokens >= 32768-4096 {
		return model.Message{}, errors.New("Ollama context budget reached; review incomplete")
	}
	return response.Message, nil
}
func (c *Client) Unload(ctx context.Context, tag string) error {
	_, err := c.post(ctx, "/api/generate", map[string]any{"model": tag, "keep_alive": 0, "stream": false})
	return err
}

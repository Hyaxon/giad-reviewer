package protocol

import "encoding/json"

type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Thinking  string     `json:"thinking,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`
}
type ToolCall struct {
	Function CallFunction `json:"function"`
}
type CallFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}
type Function struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ChatRequest struct {
	Profile  string    `json:"profile"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools"`
}

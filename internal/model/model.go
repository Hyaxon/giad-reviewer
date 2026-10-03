package model

import (
	"context"
	"github.com/hyaxon/giad/pkg/protocol"
)

// Wire types are public so independent Go agents can use the same JSON contract.
type Message = protocol.Message
type ToolCall = protocol.ToolCall
type CallFunction = protocol.CallFunction
type Tool = protocol.Tool
type Function = protocol.Function

type Provider interface {
	Chat(context.Context, string, []Message, []Tool) (Message, error)
	Unload(context.Context, string) error
}

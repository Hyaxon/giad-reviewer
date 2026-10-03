package sandbox

import (
	"context"
	"io"
)

// Command identifies a trusted installed agent. Launchers choose its environment
// and working directory; neither is supplied by the agent or PR checkout.
type Command struct {
	Path string
	Args []string
}

// Launcher owns process creation and cleanup. Cancellation must unblock protocol
// I/O. A launch failure must release resources acquired before the failure.
type Launcher interface {
	Launch(context.Context, Command) (Process, error)
}

// Process exposes only protocol streams and cleanup, keeping execution details
// out of the review session. Close must stop execution, release owned resources,
// and be safe to call more than once.
type Process interface {
	Stdin() io.Writer
	Stdout() io.Reader
	Close() error
}

// Diagnostics is optional bounded agent stderr, available after Process.Close.
// Sessions include it only on failure; it is untrusted diagnostic text.
type Diagnostics interface {
	Diagnostics() string
}

package main

import (
	"context"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestSIGTERMCancelsCommandContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support sending SIGTERM to the test process")
	}
	ctx, stop := shutdownContext(context.Background())
	defer stop()
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Fatalf("shutdown did not cancel the command: %v", ctx.Err())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("SIGTERM did not reach the command cancellation/cleanup path")
	}
}

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// TrustedHostLauncher preserves prototype execution for explicitly trusted
// agents. It provides no OS isolation and terminates only the direct child.
// It must never be used as a fallback when an isolated launcher fails.
type TrustedHostLauncher struct{}

func (TrustedHostLauncher) Launch(ctx context.Context, command Command) (_ Process, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(command.Path) {
		return nil, errors.New("agent executable must have an absolute path")
	}
	workdir, err := os.MkdirTemp("", "giad-agent-")
	if err != nil {
		return nil, err
	}
	childCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(childCtx, command.Path, command.Args...)
	cmd.Dir = workdir
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	if root := os.Getenv("SYSTEMROOT"); root != "" {
		cmd.Env = append(cmd.Env, "SYSTEMROOT="+root)
	}
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 2 * time.Second
	p := &hostProcess{cmd: cmd, cancel: cancel, workdir: workdir}
	defer func() {
		if err != nil {
			err = errors.Join(err, p.Close())
		}
	}()
	p.stdin, err = cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	p.stdout, err = cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start agent: %w", err)
	}
	p.stopClosing = context.AfterFunc(childCtx, p.closePipes)
	return p, nil
}

type hostProcess struct {
	cmd            *exec.Cmd
	cancel         context.CancelFunc
	workdir        string
	stdin          io.WriteCloser
	stdout         io.ReadCloser
	stopClosing    func() bool
	closePipesOnce sync.Once
	closeOnce      sync.Once
	closeErr       error
}

func (p *hostProcess) Stdin() io.Writer  { return p.stdin }
func (p *hostProcess) Stdout() io.Reader { return p.stdout }

func (p *hostProcess) closePipes() {
	p.closePipesOnce.Do(func() {
		if p.stdin != nil {
			_ = p.stdin.Close()
		}
		if p.stdout != nil {
			_ = p.stdout.Close()
		}
	})
}

func (p *hostProcess) Close() error {
	p.closeOnce.Do(func() {
		if p.stopClosing != nil {
			p.stopClosing()
		}
		p.cancel()
		p.closePipes()
		if p.cmd.Process != nil {
			// Accepted protocol completion governs success. Exit status is not
			// used: the session deliberately stops the agent after acceptance.
			_ = p.cmd.Wait()
		}
		p.closeErr = os.RemoveAll(p.workdir)
	})
	return p.closeErr
}

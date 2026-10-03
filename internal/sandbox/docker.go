package sandbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
)

var imageReference = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/@:-]*$`)
var imageID = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var containerID = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ValidateImage accepts a single image reference, never Docker CLI options.
func ValidateImage(image string) error {
	if len(image) > 255 || !imageReference.MatchString(image) {
		return errors.New("sandbox_image must be a Docker image name or digest")
	}
	return nil
}

// DockerLauncher executes a preinstalled agent image. Nothing from the host or
// PR checkout is mounted. Images must contain the interpreter and agent files.
type DockerLauncher struct {
	Image string
}

func (l DockerLauncher) Launch(ctx context.Context, command Command) (_ Process, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateImage(l.Image); err != nil {
		return nil, err
	}
	if !path.IsAbs(command.Path) || strings.ContainsRune(command.Path, '\\') {
		return nil, errors.New("agent executable must be an absolute Linux path inside its image")
	}
	binary, err := exec.LookPath("docker")
	if err != nil {
		return nil, errors.New("Docker is required for agent isolation; no host fallback is available")
	}
	// Resolve tags once to an immutable local image ID. Never pull during review.
	resolved, err := resolveImage(ctx, binary, l.Image)
	if err != nil {
		return nil, err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	p := &dockerProcess{binary: binary, name: "giad-agent-" + hex.EncodeToString(random[:])}
	defer func() {
		if err != nil {
			err = errors.Join(err, p.Close())
		}
	}()
	// Create is separate from start so cancellation cannot leave an unidentified
	// running container. Finish the bounded create request, then check cancellation.
	createCtx, cancelCreate := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	created, createErr := dockerOutput(createCtx, binary, dockerCreateArgs(p.name, resolved, command)...)
	cancelCreate()
	if createErr != nil {
		return nil, fmt.Errorf("create isolated agent: %w", createErr)
	}
	if !containerID.MatchString(strings.TrimSpace(string(created))) {
		return nil, errors.New("Docker returned an invalid container ID")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	childCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.cmd = exec.CommandContext(childCtx, binary, "start", "--attach", "--interactive", p.name)
	p.cmd.WaitDelay = 2 * time.Second
	p.cmd.Stderr = &p.stderr
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	p.stdout, err = p.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := p.cmd.Start(); err != nil {
		return nil, fmt.Errorf("attach isolated agent: %w", err)
	}
	p.stopClosing = context.AfterFunc(childCtx, p.closePipes)
	return p, nil
}

func resolveImage(ctx context.Context, binary, image string) (string, error) {
	inspection, err := dockerOutput(ctx, binary, "image", "inspect", image)
	if err != nil {
		return "", fmt.Errorf("inspect sandbox image (start Docker and build/pull the image first): %w", err)
	}
	var images []struct {
		ID     string `json:"Id"`
		OS     string `json:"Os"`
		Config struct{ Volumes map[string]json.RawMessage }
	}
	if err := json.Unmarshal(inspection, &images); err != nil || len(images) != 1 || !imageID.MatchString(images[0].ID) {
		return "", errors.New("Docker returned an invalid image description")
	}
	if images[0].OS != "linux" || len(images[0].Config.Volumes) != 0 {
		return "", errors.New("sandbox image must be Linux and must not declare VOLUME mounts")
	}
	return images[0].ID, nil
}

func dockerCreateArgs(name, image string, command Command) []string {
	args := []string{
		"create", "--name", name, "--pull", "never", "--interactive", "--init",
		"--network", "none", "--read-only", "--user", "65532:65532",
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges=true",
		"--pids-limit", "64", "--memory", "256m", "--memory-swap", "256m", "--cpus", "1",
		"--ipc", "private", "--cgroupns", "private", "--shm-size", "8m",
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=16m,mode=1777", "--workdir", "/tmp",
		"--log-driver", "none", "--no-healthcheck", "--entrypoint", command.Path,
		"--env", "HOME=/tmp",
	}
	// Docker client configuration can automatically inject proxy URLs containing
	// credentials. Explicit empty values suppress that inheritance for both cases.
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "FTP_PROXY", "ALL_PROXY", "NO_PROXY"} {
		args = append(args, "--env", name+"=", "--env", strings.ToLower(name)+"=")
	}
	args = append(args, image)
	return append(args, command.Args...)
}

// Keep daemon errors useful but bounded. Agent stderr never shares protocol stdout.
func dockerOutput(ctx context.Context, binary string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.WaitDelay = 2 * time.Second
	var output bytes.Buffer
	var stderr boundedLog
	cmd.Stdout = &output
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return output.Bytes(), nil
}

type boundedLog struct{ buffer bytes.Buffer }

func (b *boundedLog) String() string { return b.buffer.String() }

func (b *boundedLog) Write(data []byte) (int, error) {
	remaining := 8192 - b.buffer.Len()
	if remaining > 0 {
		_, _ = b.buffer.Write(data[:min(len(data), remaining)])
	}
	return len(data), nil
}

type dockerProcess struct {
	binary         string
	name           string
	cmd            *exec.Cmd
	cancel         context.CancelFunc
	stdin          io.WriteCloser
	stdout         io.ReadCloser
	stderr         boundedLog
	stopClosing    func() bool
	closePipesOnce sync.Once
	closeOnce      sync.Once
	closeErr       error
}

func (p *dockerProcess) Stdin() io.Writer    { return p.stdin }
func (p *dockerProcess) Stdout() io.Reader   { return p.stdout }
func (p *dockerProcess) Diagnostics() string { return strings.TrimSpace(p.stderr.String()) }

func (p *dockerProcess) closePipes() {
	p.closePipesOnce.Do(func() {
		if p.stdin != nil {
			_ = p.stdin.Close()
		}
		if p.stdout != nil {
			_ = p.stdout.Close()
		}
	})
}

func (p *dockerProcess) Close() error {
	p.closeOnce.Do(func() {
		if p.stopClosing != nil {
			p.stopClosing()
		}
		if p.cancel != nil {
			p.cancel()
		}
		p.closePipes()
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Wait()
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Removing the container kills its processes, including detached descendants.
		if _, err := dockerOutput(cleanup, p.binary, "rm", "--force", "--volumes", p.name); err != nil {
			// An interrupted create may have created nothing. Verify absence using a
			// successful daemon query; an unavailable daemon is never cleanup success.
			out, inspectErr := dockerOutput(cleanup, p.binary, "container", "ls", "--all", "--quiet", "--filter", "name=^/"+p.name+"$")
			if inspectErr != nil || strings.TrimSpace(string(out)) != "" {
				p.closeErr = fmt.Errorf("remove agent container %s: %w", p.name, errors.Join(err, inspectErr))
			}
		}
	})
	return p.closeErr
}

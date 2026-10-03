package sandbox

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/hyaxon/giad/pkg/protocol"
)

// TestProfile is trusted configuration, never read from the PR checkout.
// The image must contain /giad-test, which extracts stdin into /workspace then
// executes Command/Args. No checkout, socket, or host directory is mounted.
type TestProfile struct {
	Image          string   `toml:"image"`
	Command        string   `toml:"command"`
	Args           []string `toml:"args"`
	TimeoutSeconds int      `toml:"timeout_seconds"`
}

func (p TestProfile) Validate() error {
	if err := ValidateImage(p.Image); err != nil {
		return err
	}
	if !path.IsAbs(p.Command) || strings.ContainsAny(p.Command, "\\\x00") || len(p.Command) > 4096 {
		return errors.New("test command must be an absolute Linux executable path")
	}
	if p.TimeoutSeconds < 1 || p.TimeoutSeconds > 600 {
		return errors.New("test timeout_seconds must be between 1 and 600")
	}
	if len(p.Args) > 64 {
		return errors.New("test profile exceeds 64 arguments")
	}
	for _, arg := range p.Args {
		if len(arg) > 4096 || strings.ContainsRune(arg, '\x00') {
			return errors.New("invalid test argument")
		}
	}
	return nil
}

// TestRunner is serial and per review. Results come from container state, not
// agent claims. A failing test is data; infrastructure/cleanup failures are errors.
type TestRunner struct {
	Checkout string
	Profiles map[string]TestProfile
	Results  []protocol.TestResult
}

func (r *TestRunner) Run(parent context.Context, name string) (result protocol.TestResult, err error) {
	profile, ok := r.Profiles[name]
	if !ok {
		return result, errors.New("test profile is not approved for this agent")
	}
	if err := profile.Validate(); err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(profile.TimeoutSeconds)*time.Second)
	defer cancel()
	started := time.Now()
	result.Profile = name
	defer func() {
		result.DurationMS = time.Since(started).Milliseconds()
		if err == nil {
			r.Results = append(r.Results, result)
		}
	}()
	snapshot, err := testSnapshot(ctx, r.Checkout)
	if err != nil {
		return result, fmt.Errorf("snapshot tests: %w", err)
	}
	defer func() { err = errors.Join(err, snapshot.Close(), os.Remove(snapshot.Name())) }()
	binary, err := exec.LookPath("docker")
	if err != nil {
		return result, errors.New("Docker is required for test isolation; no host fallback is available")
	}
	resolved, err := resolveImage(ctx, binary, profile.Image)
	if err != nil {
		return result, err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return result, err
	}
	container := &dockerProcess{binary: binary, name: "giad-test-" + hex.EncodeToString(random[:])}
	defer func() { err = errors.Join(err, container.Close()) }()
	createCtx, cancelCreate := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	created, createErr := dockerOutput(createCtx, binary, testCreateArgs(container.name, resolved, name, profile)...)
	cancelCreate()
	if createErr != nil {
		return result, fmt.Errorf("create isolated tests: %w", createErr)
	}
	if !containerID.MatchString(strings.TrimSpace(string(created))) {
		return result, errors.New("Docker returned an invalid test container ID")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	cmd := exec.CommandContext(ctx, binary, "start", "--attach", "--interactive", container.name)
	cmd.WaitDelay = 2 * time.Second
	cmd.Stdin = snapshot
	var output testOutput
	// Both streams are drained even after the cap, so noisy tests cannot deadlock.
	cmd.Stdout, cmd.Stderr = &output, &output
	runErr := cmd.Run()
	result.Output, result.Truncated = output.result()
	if parent.Err() != nil {
		return result, parent.Err()
	}
	if ctx.Err() != nil {
		result.TimedOut = true
		return result, nil // deferred removal kills all descendants, not just Docker CLI
	}
	// Docker's attach exit code alone cannot distinguish a test failure from a
	// daemon/transport failure. Require an observed, stopped container state.
	stateData, err := dockerOutput(ctx, binary, "inspect", "--format", "{{json .State}}", container.name)
	if err != nil {
		return result, err
	}
	var state struct {
		Status    string
		Running   bool
		ExitCode  int
		OOMKilled bool
		Error     string
	}
	if err := json.Unmarshal(stateData, &state); err != nil {
		return result, fmt.Errorf("invalid test container state: %w", err)
	}
	if state.Status != "exited" || state.Running || state.Error != "" {
		return result, fmt.Errorf("test container did not complete: %s: %s: %v", state.Status, state.Error, runErr)
	}
	if runErr != nil {
		var exited *exec.ExitError
		if !errors.As(runErr, &exited) || exited.ExitCode() != state.ExitCode {
			return result, fmt.Errorf("test attach failed: %w", runErr)
		}
	}
	result.ExitCode = &state.ExitCode
	result.OOMKilled = state.OOMKilled
	return result, nil
}

func testCreateArgs(name, image, profileName string, profile TestProfile) []string {
	args := dockerCreateArgs(name, image, Command{Path: "/giad-test", Args: append([]string{profile.Command}, profile.Args...)})
	offset := len(args) - len(profile.Args) - 2
	// Use the same isolation as agents, with room to compile and execute tests.
	for i := 0; i < offset-1; i++ {
		switch args[i] {
		case "--memory", "--memory-swap":
			args[i+1] = "1g"
		case "--pids-limit":
			args[i+1] = "128"
		case "--tmpfs":
			args[i+1] = "/tmp:rw,exec,nosuid,nodev,size=256m,mode=1777"
		}
	}
	// Add flags before the image; the final arguments are the approved command.
	extra := []string{"--tmpfs", "/workspace:rw,exec,nosuid,nodev,size=768m,mode=1777", "--label", "giad.test-profile=" + profileName}
	return append(append(append([]string{}, args[:offset]...), extra...), args[offset:]...)
}

type testOutput struct {
	mu        sync.Mutex
	data      []byte
	truncated bool
}

func (o *testOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	remaining := 64*1024 - len(o.data)
	o.data = append(o.data, p[:min(len(p), remaining)]...)
	o.truncated = o.truncated || len(p) > remaining
	return len(p), nil
}
func (o *testOutput) result() (string, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return strings.ToValidUTF8(string(o.data), "�"), o.truncated
}

// Snapshot only regular files and directories, excluding .git at every level.
// Symlinks/special files fail explicitly instead of changing test semantics.
func testSnapshot(ctx context.Context, directory string) (_ *os.File, err error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	archive, err := os.CreateTemp("", "giad-tests-*.tar")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = archive.Close()
			_ = os.Remove(archive.Name())
		}
	}()
	writer := tar.NewWriter(&snapshotWriter{file: archive, remaining: 64 * 1024 * 1024})
	entries := 0
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		if entry.Name() == ".git" {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		entries++
		if entries > 10000 {
			return errors.New("test snapshot exceeds 10000 entries")
		}
		if strings.ContainsRune(name, '\\') {
			return fmt.Errorf("unsupported test path %q", name)
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return writer.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0755})
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("test snapshot rejects symlinks/special files: %s", name)
		}
		file, err := root.Open(name)
		if err != nil {
			return err
		}
		defer file.Close()
		info, err = file.Stat()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > 16*1024*1024 {
			return fmt.Errorf("unsupported or oversized test file: %s", name)
		}
		if err := writer.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Size: info.Size(), Mode: int64(0644 | info.Mode().Perm()&0111)}); err != nil {
			return err
		}
		_, err = io.CopyN(writer, file, info.Size())
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return archive, nil
}

type snapshotWriter struct {
	file      *os.File
	remaining int
}

func (w *snapshotWriter) Write(data []byte) (int, error) {
	if len(data) > w.remaining {
		return 0, errors.New("test snapshot exceeds 64 MiB")
	}
	n, err := w.file.Write(data)
	w.remaining -= n
	return n, err
}

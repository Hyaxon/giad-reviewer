package sandbox

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSnapshotExcludesGitAndRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{".git/config": "credential", "source.go": "package example"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	archive, err := testSnapshot(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archive.Name())
	defer archive.Close()
	reader := tar.NewReader(archive)
	h, err := reader.Next()
	if err != nil || h.Name != "source.go" {
		t.Fatalf("header=%+v err=%v", h, err)
	}
	if _, err := reader.Next(); err != io.EOF {
		t.Fatalf("unexpected snapshot entry: %v", err)
	}
	if err := os.Symlink("source.go", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := testSnapshot(context.Background(), root); err == nil || !strings.Contains(err.Error(), "symlinks") {
		t.Fatalf("symlink must fail explicitly: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := testSnapshot(ctx, root); err != context.Canceled {
		t.Fatalf("canceled snapshot: %v", err)
	}
}

func TestSnapshotRejectsOversizedFile(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "huge"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := file.Truncate(16*1024*1024 + 1); err != nil {
		t.Fatal(err)
	}
	if _, err := testSnapshot(context.Background(), filepath.Dir(file.Name())); err == nil {
		t.Fatal("oversized snapshot must fail")
	}
}

func TestDockerTestProfiles(t *testing.T) {
	image := os.Getenv("GIAD_TEST_RUNNER_IMAGE")
	if image == "" {
		t.Skip("set GIAD_TEST_RUNNER_IMAGE to exercise sandboxed Go tests")
	}
	fixture, err := os.ReadFile("../../example/go-tests/fixture/add_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, command       string
		args                []string
		timeout             int
		code                int
		timedOut, truncated bool
	}{
		{name: "failed Go test", command: "/usr/local/go/bin/go", args: []string{"test", "-p", "1", "./..."}, timeout: 90, code: 1},
		{name: "passing Go test", command: "/usr/local/go/bin/go", args: []string{"test", "-p", "1", "./..."}, timeout: 90},
		{name: "timeout", command: "/bin/sh", args: []string{"-c", "sleep 300 & wait"}, timeout: 2, timedOut: true},
		{name: "bounded output", command: "/bin/sh", args: []string{"-c", "yes noisy | head -c 100000"}, timeout: 10, truncated: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			for name, content := range map[string]string{"go.mod": "module fixture.test\n\ngo 1.27.1\n", "add.go": "package calculator\nfunc Add(a,b int) int { return a-b }\n", "add_test.go": string(fixture), ".git/config": "host-only-secret"} {
				if test.name == "passing Go test" && name == "add.go" {
					content = strings.Replace(content, "a-b", "a+b", 1)
				}
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			sentinel := filepath.Join(t.TempDir(), "host-secret")
			if err := os.WriteFile(sentinel, []byte("secret"), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GH_TOKEN", "host-only-secret")
			probe := fmt.Sprintf(`package calculator
import ("net";"os";"testing";"time")
func TestIsolation(t *testing.T) {
 if os.Getuid()!=65532 || os.Getenv("GH_TOKEN")!="" { t.Fatal("host identity inherited") }
 for _,p:=range []string{".git/config","/var/run/docker.sock",%q} {
  if _,err:=os.Stat(p);!os.IsNotExist(err) { t.Fatalf("host path visible: %%s: %%v",p,err) }
 }
 if f,err:=os.OpenFile("/usr/local/go/VERSION",os.O_WRONLY,0);err==nil { f.Close();t.Fatal("image writable") }
 if c,err:=net.DialTimeout("tcp","1.1.1.1:443",time.Second);err==nil { c.Close();t.Fatal("network available") }
 if err:=os.WriteFile("scratch",[]byte("container-only"),0600);err!=nil { t.Fatal(err) }
}
`, sentinel)
			if err := os.WriteFile(filepath.Join(root, "isolation_test.go"), []byte(probe), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			binary, err := exec.LookPath("docker")
			if err != nil {
				t.Fatal(err)
			}
			before, err := dockerOutput(ctx, binary, "container", "ls", "--all", "--quiet", "--filter", "label=giad.test-profile=fixture")
			if err != nil {
				t.Fatal(err)
			}
			runner := TestRunner{Checkout: root, Profiles: map[string]TestProfile{"fixture": {Image: image, Command: test.command, Args: test.args, TimeoutSeconds: test.timeout}}}
			result, err := runner.Run(ctx, "fixture")
			if err != nil || result.TimedOut != test.timedOut || result.Truncated != test.truncated || result.DurationMS <= 0 || len(runner.Results) != 1 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if test.timedOut {
				if result.ExitCode != nil {
					t.Fatal("timeout must not claim an exit code")
				}
			} else if result.ExitCode == nil || *result.ExitCode != test.code {
				t.Fatalf("exit=%v output=%s", result.ExitCode, result.Output)
			}
			if test.name == "failed Go test" && !strings.Contains(result.Output, "Add(2, 3)") {
				t.Fatalf("failure output lost: %s", result.Output)
			}
			if _, err := os.Stat(filepath.Join(root, "scratch")); !os.IsNotExist(err) {
				t.Fatal("container changed host checkout")
			}
			after, err := dockerOutput(ctx, binary, "container", "ls", "--all", "--quiet", "--filter", "label=giad.test-profile=fixture")
			if err != nil || string(before) != string(after) {
				t.Fatalf("test container leaked: before=%s after=%s err=%v", before, after, err)
			}
			if _, err := runner.Run(ctx, "unapproved"); err == nil || len(runner.Results) != 1 {
				t.Fatal("unapproved profile was executed")
			}
		})
	}
}

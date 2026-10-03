package sandbox

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func dockerTestImage(t *testing.T) string {
	t.Helper()
	image := os.Getenv("GIAD_TEST_DOCKER_IMAGE")
	if image == "" {
		t.Skip("set GIAD_TEST_DOCKER_IMAGE to run real container isolation checks")
	}
	return image
}

func TestDockerIsolationAndCleanup(t *testing.T) {
	image := dockerTestImage(t)
	t.Setenv("GH_TOKEN", "host-only-secret")
	sentinel := filepath.Join(t.TempDir(), "host-secret")
	if err := os.WriteFile(sentinel, []byte("must not be visible"), 0600); err != nil {
		t.Fatal(err)
	}
	script := `import json, os, socket, subprocess, sys, time
def writable(path):
    try:
        with open(path, "w") as f: f.write("probe")
        return True
    except OSError: return False
try:
    socket.create_connection(("1.1.1.1", 443), timeout=0.5).close()
    network = True
except OSError: network = False
child = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(300)"], start_new_session=True)
print(json.dumps({"uid": os.getuid(), "token": os.getenv("GH_TOKEN", ""),
    "host_visible": os.path.exists(sys.argv[1]), "socket_visible": os.path.exists("/var/run/docker.sock"),
    "scratch_write": writable("/tmp/probe"), "image_write": writable("/agent/agent.py"),
    "network": network, "descendant": child.pid}), flush=True)
time.sleep(300)
`
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	process, err := (DockerLauncher{Image: image}).Launch(ctx, Command{
		Path: "/usr/local/bin/python3", Args: []string{"-c", script, sentinel},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := process.Close(); err != nil {
			t.Error(err)
		}
	})
	var observed struct {
		UID           int    `json:"uid"`
		Token         string `json:"token"`
		HostVisible   bool   `json:"host_visible"`
		SocketVisible bool   `json:"socket_visible"`
		ScratchWrite  bool   `json:"scratch_write"`
		ImageWrite    bool   `json:"image_write"`
		Network       bool   `json:"network"`
		Descendant    int    `json:"descendant"`
	}
	if err := json.NewDecoder(process.Stdout()).Decode(&observed); err != nil {
		t.Fatal(err)
	}
	if observed.UID != 65532 || observed.Token != "" || observed.HostVisible || observed.SocketVisible || !observed.ScratchWrite || observed.ImageWrite || observed.Network || observed.Descendant <= 1 {
		t.Fatalf("isolation probe: %+v", observed)
	}
	p := process.(*dockerProcess)
	// Verify the daemon applied the limits, rather than only checking CLI arguments.
	inspection, err := dockerOutput(ctx, p.binary, "container", "inspect", p.name)
	if err != nil {
		t.Fatal(err)
	}
	var containers []struct {
		HostConfig struct {
			ReadonlyRootfs bool
			NetworkMode    string
			Memory         int64
			MemorySwap     int64
			NanoCpus       int64
			PidsLimit      int64
			Binds          []string
			CapDrop        []string
			SecurityOpt    []string
		}
		Mounts []json.RawMessage
	}
	if err := json.Unmarshal(inspection, &containers); err != nil || len(containers) != 1 {
		t.Fatalf("inspect container: %s (%v)", inspection, err)
	}
	policy := containers[0].HostConfig
	if !policy.ReadonlyRootfs || policy.NetworkMode != "none" || policy.Memory != 256*1024*1024 || policy.MemorySwap != policy.Memory || policy.NanoCpus != 1_000_000_000 || policy.PidsLimit != 64 || len(policy.Binds) != 0 || len(containers[0].Mounts) != 0 || len(policy.CapDrop) != 1 || policy.CapDrop[0] != "ALL" || len(policy.SecurityOpt) != 1 || policy.SecurityOpt[0] != "no-new-privileges=true" {
		t.Fatalf("daemon did not apply isolation policy: %+v mounts=%v", policy, containers[0].Mounts)
	}
	done := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, process.Stdout()); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation left protocol read blocked")
	}
	for i := 0; i < 2; i++ {
		if err := process.Close(); err != nil {
			t.Fatal(err)
		}
	}
	cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	remaining, err := dockerOutput(cleanup, p.binary, "container", "ls", "--all", "--quiet", "--filter", "name=^/"+p.name+"$")
	if err != nil || strings.TrimSpace(string(remaining)) != "" {
		t.Fatalf("container survived cleanup: %s (%v)", remaining, err)
	}
}

func TestDockerMissingImageFails(t *testing.T) {
	_ = dockerTestImage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, err := (DockerLauncher{Image: "giad-intentionally-missing-image:no-pull"}).Launch(ctx, Command{Path: "/agent/agent"})
	if err == nil || p != nil || !strings.Contains(err.Error(), "inspect sandbox image") {
		t.Fatalf("missing image must fail without pulling or executing: %v %v", p, err)
	}
}

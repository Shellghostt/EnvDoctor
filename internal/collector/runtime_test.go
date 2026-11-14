package collector_test

import (
	"context"
	"github.com/shellghostt/envdoctor/internal/collector"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRuntimeDiscoveryDoesNotExecutePATHBinaries(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "nvm", "v22.11.0", "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	name := "node"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte("not an executable; inspection must not run this"), 0755); err != nil {
		t.Fatal(err)
	}
	c := collector.RuntimeCollector{Getenv: func(key string) string {
		if key == "PATH" {
			return bin
		}
		return ""
	}, GOOS: runtime.GOOS}
	p, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Runtimes) != 1 || p.Runtimes[0].Version != "22.11.0" || p.Runtimes[0].Manager != "nvm" || !p.Runtimes[0].Active {
		t.Fatalf("%+v", p.Runtimes)
	}
}
func TestPortCollectorFindsOwnedListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	port := listener.Addr().(*net.TCPAddr).Port
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	p, err := (collector.PortCollector{Requested: []int{port}}).Collect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range p.Ports {
		if p.Port == port && p.PID == os.Getpid() && p.Process != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("own listener was not attributed: %+v", p.Ports)
	}
}

func TestRuntimeDoesNotInferVersionFromUnrelatedDirectory(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "1.2.3", "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	name := "node"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte("metadata without a version"), 0755); err != nil {
		t.Fatal(err)
	}
	c := collector.RuntimeCollector{Getenv: func(key string) string {
		if key == "PATH" {
			return bin
		}
		return ""
	}}
	p, err := c.Collect(context.Background())
	if err != nil || len(p.Runtimes) != 1 || p.Runtimes[0].Version != "" {
		t.Fatalf("unrelated directory became a runtime version: %+v %v", p.Runtimes, err)
	}
}

func TestBrokenSymlinkDoesNotShadowUsableRuntime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable PATH resolution; Windows symlink permissions differ")
	}
	root := t.TempDir()
	first, second := filepath.Join(root, "first"), filepath.Join(root, "nvm", "v22.11.0", "bin")
	for _, dir := range []string{first, second} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(first, "node")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "node"), []byte("metadata"), 0755); err != nil {
		t.Fatal(err)
	}
	c := collector.RuntimeCollector{Getenv: func(key string) string {
		if key == "PATH" {
			return first + string(os.PathListSeparator) + second
		}
		return ""
	}}
	p, err := c.Collect(context.Background())
	if err != nil || len(p.Runtimes) != 2 || p.Runtimes[0].Active || !p.Runtimes[1].Active {
		t.Fatalf("broken symlink shadowed a usable executable: %+v %v", p.Runtimes, err)
	}
}

func TestVirtualEnvironmentIsDistinctFromItsSystemSymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix Python venv symlink semantics")
	}
	root := t.TempDir()
	systemBin, venvBin := filepath.Join(root, "system", "bin"), filepath.Join(root, "venv", "bin")
	for _, dir := range []string{systemBin, venvBin} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	python := filepath.Join(systemBin, "python")
	if err := os.WriteFile(python, []byte("metadata"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(python, filepath.Join(venvBin, "python")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "venv", "pyvenv.cfg"), []byte("version = 3.12.7\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c := collector.RuntimeCollector{Getenv: func(key string) string {
		switch key {
		case "PATH":
			return systemBin + string(os.PathListSeparator) + venvBin
		case "VIRTUAL_ENV":
			return filepath.Join(root, "venv")
		}
		return ""
	}}
	p, err := c.Collect(context.Background())
	if err != nil || len(p.Runtimes) != 2 || p.Runtimes[0].Expected || !p.Runtimes[1].Expected || p.Runtimes[1].Active || p.Runtimes[1].Manager != "venv" {
		t.Fatalf("venv merged into its system interpreter: %+v %v", p.Runtimes, err)
	}
}

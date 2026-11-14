package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

type processExit int

func (e processExit) Error() string { return fmt.Sprintf("exit %d", e) }
func (e processExit) ExitCode() int { return int(e) }

func TestLSOFOutputAndFailureDistinction(t *testing.T) {
	got, err := parseLSOF([]byte("p42\ncnode\nn[::1]:3000\np57\ncpython\nn127.0.0.1:8000\n"), nil)
	want := []Socket{{Port: 3000, PID: 42, Process: "node"}, {Port: 8000, PID: 57, Process: "python"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("parsed sockets=%+v err=%v", got, err)
	}
	if got, err := parseLSOF(nil, processExit(1)); err != nil || len(got) != 0 {
		t.Fatalf("normal empty lsof selection: %+v %v", got, err)
	}
	for _, failure := range []error{os.ErrNotExist, context.DeadlineExceeded, processExit(2)} {
		if _, err := parseLSOF(nil, failure); !errors.Is(err, failure) {
			t.Fatalf("inspection failure became a clean result: %v", err)
		}
	}
}

func TestInspectionHelper(t *testing.T) {
	if os.Getenv("ENVDOCTOR_INSPECTION_HELPER") != "1" {
		return
	}
	if os.Getenv("ENVDOCTOR_HELPER_MODE") == "wait" {
		time.Sleep(10 * time.Second)
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"git": os.Getenv("GIT_DIR"), "docker": os.Getenv("DOCKER_CONTEXT")})
	os.Exit(0)
}

func TestRunToolSanitizesContextOverrides(t *testing.T) {
	t.Setenv("ENVDOCTOR_INSPECTION_HELPER", "1")
	t.Setenv("GIT_DIR", "injected-repository")
	t.Setenv("DOCKER_CONTEXT", "remote-collector-must-not-contact")
	out, err := RunTool(context.Background(), os.Args[0], "-test.run=^TestInspectionHelper$")
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]string
	if err = json.Unmarshal(out, &values); err != nil || values["git"] != "" || values["docker"] != "" {
		t.Fatalf("context overrides reached child process: %s %v", out, err)
	}
}

func TestRunToolHonorsDeadline(t *testing.T) {
	t.Setenv("ENVDOCTOR_INSPECTION_HELPER", "1")
	t.Setenv("ENVDOCTOR_HELPER_MODE", "wait")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := RunTool(ctx, os.Args[0], "-test.run=^TestInspectionHelper$"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline not reported: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("tool process exceeded bounded shutdown")
	}
}

func TestInstalledDockerIncludesPerUserWindowsLocation(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows installation locations")
	}
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	t.Setenv("ProgramFiles", filepath.Join(root, "unused-programs"))
	path := filepath.Join(root, "Programs", "DockerDesktop", "resources", "bin", "docker.exe")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("inspection does not execute this file"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := InstalledTool("docker"); got != path {
		t.Fatalf("per-user Docker was missed: %q", got)
	}
}

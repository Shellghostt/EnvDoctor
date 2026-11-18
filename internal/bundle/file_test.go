package bundle

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/report"
)

func publicationArchive(t *testing.T) []byte {
	t.Helper()
	stamp := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	doc := report.NewDocument("0.1.0", stamp, collector.Snapshot{SchemaVersion: 1, CollectedAt: stamp}, nil, nil, 12)
	data, err := Build(context.Background(), doc, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestM4PublicationEveryStagePreservesDestination(t *testing.T) {
	data := publicationArchive(t)
	for _, stage := range []string{"protect", "write", "short-write", "sync", "close", "read", "validate", "rename", "cancel-after-write", "cancel-before-rename"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "capture.zip")
			previous := []byte("previous regular destination")
			if err := os.WriteFile(path, previous, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fail := errors.New("SECRET-STAGING-ERROR")
			steps := publicationSteps{protect: privateFile, write: func(f *os.File, p []byte) (int, error) { return f.Write(p) }, sync: func(f *os.File) error { return f.Sync() }, close: func(f *os.File) error { return f.Close() }, read: readFile, replace: replaceFile}
			switch stage {
			case "protect":
				steps.protect = func(*os.File) error { return fail }
			case "write":
				steps.write = func(f *os.File, p []byte) (int, error) { n, _ := f.Write(p[:len(p)/2]); return n, fail }
			case "short-write":
				steps.write = func(f *os.File, p []byte) (int, error) { return f.Write(p[:len(p)/2]) }
			case "sync":
				steps.sync = func(*os.File) error { return fail }
			case "close":
				steps.close = func(f *os.File) error { _ = f.Close(); return fail }
			case "read":
				steps.read = func(context.Context, string) ([]byte, error) { return nil, ErrRead }
			case "validate":
				steps.read = func(context.Context, string) ([]byte, error) { return []byte("SECRET-ALTERED-STAGING"), nil }
			case "rename":
				steps.replace = func(string, string) error { return fail }
			case "cancel-after-write":
				steps.write = func(f *os.File, p []byte) (int, error) { n, err := f.Write(p); cancel(); return n, err }
			case "cancel-before-rename":
				steps.read = func(ctx context.Context, path string) ([]byte, error) {
					p, err := readFile(ctx, path)
					cancel()
					return p, err
				}
			}
			err := publish(ctx, path, data, steps)
			if err == nil || bytes.Contains([]byte(err.Error()), []byte("SECRET-")) {
				t.Fatalf("unsafe failure: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, previous) {
				t.Fatal("previous destination replaced")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatal("owned staging file retained")
			}
		})
	}
}

func TestM4SymlinkDestinationAndInputAreNotFollowed(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.zip")
	link := filepath.Join(dir, "link.zip")
	data := publicationArchive(t)
	if err := os.WriteFile(target, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink capability unavailable: %v", err)
	}
	if err := Publish(context.Background(), link, data); !errors.Is(err, ErrWrite) {
		t.Fatalf("symlink output: %v", err)
	}
	if _, err := Load(context.Background(), link); !errors.Is(err, ErrRead) {
		t.Fatalf("symlink input: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("symlink target changed")
	}
}

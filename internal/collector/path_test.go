package collector_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shellghostt/envdoctor/internal/collector"
)

type fakeFileInfo struct {
	name  string
	isDir bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.isDir }
func (f fakeFileInfo) Sys() any           { return nil }

// statFrom returns a Stat function backed by a set of existing directories.
func statFrom(existing map[string]bool) func(string) (os.FileInfo, error) {
	return func(path string) (os.FileInfo, error) {
		if !existing[path] {
			return nil, os.ErrNotExist
		}
		return fakeFileInfo{name: path, isDir: true}, nil
	}
}

func TestPathCollectorReportsMissingAndDuplicates(t *testing.T) {
	separator := string(os.PathListSeparator)
	value := "/opt/a" + separator + "/opt/gone" + separator + "/opt/a"

	c := collector.PathCollector{
		Getenv:    func(string) string { return value },
		Stat:      statFrom(map[string]bool{"/opt/a": true}),
		Separator: separator,
	}
	part, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if part.Path == nil {
		t.Fatal("path part missing")
	}

	entries := part.Path.Entries
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3: %+v", len(entries), entries)
	}

	if !entries[0].Exists || !entries[0].IsDir || entries[0].Position != 0 || entries[0].Duplicate {
		t.Errorf("first entry = %+v", entries[0])
	}
	if entries[1].Exists {
		t.Errorf("missing entry reported as existing: %+v", entries[1])
	}
	if entries[1].Raw != "/opt/gone" {
		t.Errorf("raw entry was rewritten: %q", entries[1].Raw)
	}
	if !entries[2].Duplicate {
		t.Errorf("duplicate not marked: %+v", entries[2])
	}
	if entries[2].Position != 2 {
		t.Errorf("position = %d, want 2", entries[2].Position)
	}
}

func TestPathCollectorSkipsEmptyEntries(t *testing.T) {
	c := collector.PathCollector{
		Getenv:    func(string) string { return ";;" },
		Stat:      statFrom(nil),
		Separator: ";",
	}
	part, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(part.Path.Entries) != 0 {
		t.Fatalf("empty entries were recorded: %+v", part.Path.Entries)
	}
}

func TestPathCollectorUnquotesForStatOnly(t *testing.T) {
	var statted string
	c := collector.PathCollector{
		Getenv: func(string) string { return `"C:\Program Files\bin"` },
		Stat: func(path string) (os.FileInfo, error) {
			statted = path
			return fakeFileInfo{isDir: true}, nil
		},
		Separator: ";",
	}

	part, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if statted != `C:\Program Files\bin` {
		t.Fatalf("stat path = %q, quotes were not removed", statted)
	}
	if part.Path.Entries[0].Raw != `"C:\Program Files\bin"` {
		t.Fatalf("raw entry lost its quotes: %q", part.Path.Entries[0].Raw)
	}
}

func TestPathCollectorNormalizesDuplicates(t *testing.T) {
	c := collector.PathCollector{
		Getenv:     func(string) string { return "/opt/bin:/opt/BIN" },
		Stat:       statFrom(map[string]bool{"/opt/bin": true, "/opt/BIN": true}),
		Separator:  ":",
		Normalizer: func(string) string { return "same" },
	}
	part, _ := c.Collect(context.Background())

	if !part.Path.Entries[1].Duplicate {
		t.Fatalf("normalized duplicate was not marked: %+v", part.Path.Entries[1])
	}
}

func TestPathCollectorStopsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := collector.PathCollector{
		Getenv:    func(string) string { return "/a:/b" },
		Stat:      statFrom(map[string]bool{"/a": true, "/b": true}),
		Separator: ":",
	}
	if _, err := c.Collect(ctx); err == nil {
		t.Fatal("cancelled context should return an error")
	}
}

func TestPathPermissionFailureIsNotMissingDirectoryEvidence(t *testing.T) {
	c := collector.PathCollector{Getenv: func(string) string { return "/private/bin" }, Separator: ":", Stat: func(string) (os.FileInfo, error) { return nil, os.ErrPermission }}
	if _, err := c.Collect(context.Background()); err == nil {
		t.Fatal("access denied was converted into evidence that a PATH directory is missing")
	}
}

func TestPathThroughRegularFileIsReportedAsMissing(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	raw := filepath.Join(file, "bin")
	c := collector.PathCollector{Getenv: func(string) string { return raw }, Separator: string(os.PathListSeparator), Stat: os.Stat}
	part, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("non-directory ancestor degraded collection: %v", err)
	}
	if part.Path == nil || len(part.Path.Entries) != 1 || part.Path.Entries[0].Raw != raw || part.Path.Entries[0].Exists {
		t.Fatalf("invalid raw entry was not preserved as missing: %+v", part.Path)
	}
}

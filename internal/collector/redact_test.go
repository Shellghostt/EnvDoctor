package collector_test

import (
	"testing"

	"github.com/shellghostt/envdoctor/internal/collector"
)

func TestRedactHomePrefix(t *testing.T) {
	cases := map[string]string{
		`C:\Users\ann\bin`:        `~\bin`,
		`C:\Users\ann`:            `~`,
		`C:\Users\anna\bin`:       `C:\Users\anna\bin`,
		`C:\Users\ann\bin2`:       `~\bin2`,
		`/home/ann/bin`:           `~/bin`,
		`/home/ann` + string('/'): `~/`,
		`/other/ann/bin`:          `/other/ann/bin`,
		`C:/Users/ann/bin`:        `~/bin`,
	}

	for input, want := range cases {
		home := `C:\Users\ann`
		if input[0] == '/' {
			home = "/home/ann"
		}
		if got := collector.RedactHome(input, home); got != want {
			t.Errorf("RedactHome(%q, %q) = %q, want %q", input, home, got, want)
		}
	}
}

func TestRedactHomeIsCaseInsensitive(t *testing.T) {
	// Windows paths are case-insensitive; a false negative would leak a
	// username into a shared snapshot.
	if got := collector.RedactHome(`c:\users\ANN\bin`, `C:\Users\Ann`); got != `~\bin` {
		t.Fatalf("got %q", got)
	}
}

func TestRedactEmptyHomeIsNoop(t *testing.T) {
	snapshot := &collector.Snapshot{
		Shell: &collector.Shell{Name: "bash", Path: "/home/ann/.bashrc"},
		Path: &collector.PathInfo{Entries: []collector.PathEntry{
			{Raw: "/home/ann/bin", Exists: true, IsDir: true},
		}},
	}

	snapshot.Redact("")

	if snapshot.Shell.Path != "/home/ann/.bashrc" {
		t.Errorf("shell path changed with an empty home: %q", snapshot.Shell.Path)
	}
	if snapshot.Path.Entries[0].Raw != "/home/ann/bin" {
		t.Errorf("path entry changed with an empty home: %q", snapshot.Path.Entries[0].Raw)
	}
}

func TestRedactRewritesSnapshot(t *testing.T) {
	snapshot := &collector.Snapshot{
		Shell: &collector.Shell{Name: "bash", Path: "/home/ann/.bashrc"},
		Path: &collector.PathInfo{Entries: []collector.PathEntry{
			{Raw: "/home/ann/bin", Exists: true, IsDir: true},
			{Raw: "/usr/local/bin", Exists: true, IsDir: true},
		}},
		Runtimes: []collector.Runtime{{Name: "node", Path: "/home/ann/.nvm/versions/node"}},
	}

	snapshot.Redact("/home/ann")

	if snapshot.Shell.Path != "~/.bashrc" {
		t.Errorf("shell path = %q", snapshot.Shell.Path)
	}
	if snapshot.Path.Entries[0].Raw != "~/bin" {
		t.Errorf("first entry = %q", snapshot.Path.Entries[0].Raw)
	}
	if snapshot.Path.Entries[1].Raw != "/usr/local/bin" {
		t.Errorf("unrelated entry was rewritten: %q", snapshot.Path.Entries[1].Raw)
	}
	if snapshot.Runtimes[0].Path != "~/.nvm/versions/node" {
		t.Errorf("runtime path = %q", snapshot.Runtimes[0].Path)
	}
	// Existence is decided before redaction and must survive it.
	if !snapshot.Path.Entries[0].Exists {
		t.Error("redaction changed existence")
	}
}

package report_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/shellghostt/envdoctor/internal/report"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("synthetic write failure") }

func TestM4FormatPolicy(t *testing.T) {
	tty := report.TerminalState{Input: true, Output: true, Error: true, Term: "xterm-256color"}
	for _, tc := range []struct {
		name string
		opts report.FormatOptions
		term report.TerminalState
		want report.Mode
		bad  bool
	}{
		{"interactive", report.FormatOptions{}, tty, report.Interactive, false},
		{"no-terminal", report.FormatOptions{}, report.TerminalState{}, report.Text, false},
		{"redirect-stdout", report.FormatOptions{}, report.TerminalState{Input: true, Error: true}, report.Text, false},
		{"redirect-stderr", report.FormatOptions{}, report.TerminalState{Input: true, Output: true}, report.Text, false},
		{"redirect-stdin", report.FormatOptions{}, report.TerminalState{Output: true, Error: true}, report.Text, false},
		{"dumb", report.FormatOptions{}, report.TerminalState{Input: true, Output: true, Error: true, Term: "dumb"}, report.Text, false},
		{"no-color-even-empty", report.FormatOptions{}, report.TerminalState{Input: true, Output: true, Error: true, NoColor: true}, report.Text, false},
		{"windows-no-term", report.FormatOptions{}, report.TerminalState{Input: true, Output: true, Error: true}, report.Interactive, false},
		{"plain", report.FormatOptions{Plain: true}, tty, report.Text, false},
		{"explicit-text", report.FormatOptions{Format: "text", Explicit: true}, tty, report.Text, false},
		{"json-alias", report.FormatOptions{JSON: true}, tty, report.JSON, false},
		{"json", report.FormatOptions{Format: "json", Explicit: true}, tty, report.JSON, false},
		{"markdown", report.FormatOptions{Format: "markdown", Explicit: true}, tty, report.Markdown, false},
		{"compatible-json", report.FormatOptions{JSON: true, Format: "json", Explicit: true}, tty, report.JSON, false},
		{"conflicting-alias", report.FormatOptions{JSON: true, Format: "markdown", Explicit: true}, tty, "", true},
		{"plain-json", report.FormatOptions{JSON: true, Plain: true}, tty, "", true},
		{"plain-markdown", report.FormatOptions{Plain: true, Format: "markdown", Explicit: true}, tty, "", true},
		{"unknown", report.FormatOptions{Format: "html", Explicit: true}, tty, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := report.SelectMode(tc.opts, tc.term)
			if (err != nil) != tc.bad || !tc.bad && got != tc.want {
				t.Fatalf("mode=%q, err=%v; want=%q, bad=%v", got, err, tc.want, tc.bad)
			}
		})
	}
}

func markdownDocument() report.Document {
	s := collector.Snapshot{SchemaVersion: 1, CollectedAt: fixedTime,
		Platform: &collector.Platform{OS: "windows", Arch: "amd64"},
		Errors:   []collector.CollectorError{{Collector: "git", Message: "captured collector unavailable"}}}
	findings := []diagnostic.Finding{
		{RuleID: "ENV900", Severity: diagnostic.SeverityCritical, Component: "git", Title: "Critical evidence",
			Detected: []string{"<script>bad</script>|[x](https://invalid.example)"}, Cause: "Unknown cause",
			Fix: &diagnostic.Fix{Summary: "Check manually", Commands: []string{"echo `test`"}}, Verify: "echo \"ok\""},
		{RuleID: "ENV901", Severity: diagnostic.SeverityError, Component: "git", Title: "Error evidence", Detected: []string{"error"}},
		{RuleID: "ENV902", Severity: diagnostic.SeverityWarning, Component: "git", Title: "Warning evidence", Detected: []string{"warning"}},
		{RuleID: "ENV903", Severity: diagnostic.SeverityInfo, Component: "git", Title: "Info evidence", Detected: []string{"info"}},
	}
	findings[0].Detected = append(findings[0].Detected, `C:\synthetic\project with spaces\`+strings.Repeat(`long-directory-name-0123456789\`, 8)+`<end>|file.txt`)
	doc := report.NewDocument("0.1.0", fixedTime, s, findings, nil, 12)
	doc.Source = &report.Source{Kind: "bundle", CaptureTool: report.Tool{Name: "envdoctor", Version: "0.1.0"},
		CapturedAt: fixedTime, RedactionPolicy: "allowlist-v1", CaptureExitCode: 2,
		Omissions: []report.Omission{{Field: "git.identity", Reason: "identifying_values"}}}
	return doc
}

func TestM4MarkdownBoundsAndBidiControls(t *testing.T) {
	doc := markdownDocument()
	doc.Findings[0].Detected = []string{"visible\u202Eafter"}
	var out bytes.Buffer
	if err := report.WriteMarkdown(&out, doc); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(out.String(), '\u202E') || !strings.Contains(out.String(), "u202E") {
		t.Fatal("bidi control not made visible")
	}
	doc.Findings[0].Detected = []string{strings.Repeat("!", 1<<20)}
	out.Reset()
	if err := report.WriteMarkdown(&out, doc); err == nil || out.Len() != 0 {
		t.Fatal("oversized Markdown partially written or accepted")
	}
}

func TestM4MarkdownGoldenAndEscaping(t *testing.T) {
	var out bytes.Buffer
	if err := report.WriteMarkdown(&out, markdownDocument()); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "markdown-m4.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(want) {
		t.Fatalf("Markdown differs from reviewed golden:\n%s", out.String())
	}
	for _, forbidden := range []string{"<script>", "[x](", "https://", "`test`", "\x1b"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("active or control text retained: %q", forbidden)
		}
	}
}

func TestM4TextControlsAndMarkdownWriteFailure(t *testing.T) {
	doc := markdownDocument()
	doc.Findings[0].Detected = []string{"before\x1b[2J\r\x00after"}
	var out bytes.Buffer
	if err := report.WriteText(&out, doc); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out.String(), "\x1b\r\x00") || !strings.Contains(out.String(), "after") {
		t.Fatalf("unsafe or lost plain evidence: %q", out.String())
	}
	if err := report.WriteMarkdown(failingWriter{}, doc); err == nil {
		t.Fatal("Markdown write failure ignored")
	}
}

type shortReportWriter struct{}

func (shortReportWriter) Write(p []byte) (int, error) { return max(0, len(p)-1), nil }

func TestM4EveryFormatPropagatesShortWrites(t *testing.T) {
	for _, format := range []struct {
		name  string
		write func(io.Writer, report.Document) error
	}{{"text", report.WriteText}, {"markdown", report.WriteMarkdown}, {"json", report.WriteJSON}} {
		t.Run(format.name, func(t *testing.T) {
			if err := format.write(shortReportWriter{}, markdownDocument()); !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("short write was not propagated: %v", err)
			}
		})
	}
}

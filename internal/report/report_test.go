package report_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/shellghostt/envdoctor/internal/report"
)

var fixedTime = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

func snapshot() collector.Snapshot {
	return collector.Snapshot{
		SchemaVersion: 1,
		CollectedAt:   fixedTime,
		Platform: &collector.Platform{
			OS:       "windows",
			Version:  "11 (build 22631)",
			Arch:     "amd64",
			Elevated: false,
		},
		Shell: &collector.Shell{Name: "powershell"},
		Path: &collector.PathInfo{Entries: []collector.PathEntry{
			{Raw: `~\bin`, Exists: true, IsDir: true, Position: 0},
			{Raw: `~\gone`, Exists: false, Position: 1},
			{Raw: `~\bin`, Exists: true, IsDir: true, Position: 2, Duplicate: true},
		}},
	}
}

func timings() []collector.Timing {
	return []collector.Timing{
		{Name: "platform", Duration: 2 * time.Millisecond},
		{Name: "path", Duration: 5 * time.Millisecond},
	}
}

func TestTextReportShowsFactsAndPathHealth(t *testing.T) {
	document := report.NewDocument("0.1.0", fixedTime, snapshot(), nil, timings(), 0)

	var out bytes.Buffer
	if err := report.WriteText(&out, document); err != nil {
		t.Fatalf("write: %v", err)
	}
	text := out.String()

	for _, want := range []string{
		"EnvDoctor 0.1.0",
		"Platform   windows 11 (build 22631) amd64",
		"Elevated   no",
		"Shell      powershell",
		"PATH       3 entries, 1 missing, 1 duplicate",
		"Missing PATH entries:",
		"  - ~\\gone",
		"Rules      0 registered in this build",
		"No findings.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text report is missing %q:\n%s", want, text)
		}
	}

	if strings.ContainsRune(text, 0x1b) {
		t.Error("text report contains an ANSI escape sequence")
	}
}

func TestTextReportRendersFinding(t *testing.T) {
	findings := []diagnostic.Finding{{
		RuleID:    "ENV006",
		Severity:  diagnostic.SeverityError,
		Component: "docker",
		Title:     "Docker daemon unavailable",
		Detected:  []string{"Docker CLI installed (29.9.2)", "Docker daemon not responding"},
		Cause:     "Docker Desktop is not running.",
		Fix: &diagnostic.Fix{
			Summary:   "Start Docker Desktop and retry.",
			Commands:  []string{"Start-Process 'Docker Desktop.exe'"},
			Automatic: true, Reversible: true,
		},
		Verify: "envdoctor diagnose --component docker",
	}}
	document := report.NewDocument("0.2.0", fixedTime, snapshot(), findings, timings(), 18)

	var out bytes.Buffer
	if err := report.WriteText(&out, document); err != nil {
		t.Fatalf("write: %v", err)
	}
	text := out.String()

	for _, want := range []string{
		"[ERROR] Docker daemon unavailable",
		"Detected:",
		"  - Docker CLI installed (29.9.2)",
		"Possible cause:",
		"  Docker Desktop is not running.",
		"Suggested fix:",
		"Run:",
		"  Start-Process 'Docker Desktop.exe'",
		"Rule: ENV006",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("finding render is missing %q:\n%s", want, text)
		}
	}
}

func TestTextReportShowsDegradedCollector(t *testing.T) {
	document := report.NewDocument("0.1.0", fixedTime, snapshot(), nil, []collector.Timing{
		{Name: "platform", Duration: time.Millisecond},
		{Name: "path", Duration: 2 * time.Second, Err: errTimeout{}},
	}, 0)

	if document.ExitCode != 3 {
		t.Fatalf("exit code = %d, want 3 for a degraded run", document.ExitCode)
	}

	var out bytes.Buffer
	if err := report.WriteText(&out, document); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(out.String(), "Degraded collectors") {
		t.Fatalf("degraded notice missing:\n%s", out.String())
	}
}

type errTimeout struct{}

func (errTimeout) Error() string { return "timeout after 2s" }

func TestJSONReportIsOneDocument(t *testing.T) {
	document := report.NewDocument("0.1.0", fixedTime, snapshot(), nil, timings(), 0)

	var out bytes.Buffer
	if err := report.WriteJSON(&out, document); err != nil {
		t.Fatalf("write: %v", err)
	}

	var decoded report.Document
	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		t.Fatalf("output is not a single decodable document: %v\n%s", err, out.String())
	}
	if decoder.More() {
		t.Fatal("output contains more than one document")
	}

	if decoded.Findings == nil {
		t.Error("findings must render as [] rather than null")
	}
	if decoded.Summary.Total != 0 {
		t.Errorf("summary = %+v", decoded.Summary)
	}
	if decoded.ExitCode != 0 {
		t.Errorf("exit code = %d", decoded.ExitCode)
	}
}

func TestNewDocumentCountsSeverities(t *testing.T) {
	findings := []diagnostic.Finding{
		{RuleID: "ENV001", Severity: diagnostic.SeverityInfo},
		{RuleID: "ENV002", Severity: diagnostic.SeverityWarning},
		{RuleID: "ENV003", Severity: diagnostic.SeverityError},
		{RuleID: "ENV004", Severity: diagnostic.SeverityCritical},
	}
	document := report.NewDocument("0.1.0", fixedTime, snapshot(), findings, timings(), 18)

	if document.Summary != (report.Summary{Total: 4, Info: 1, Warning: 1, Error: 1, Critical: 1}) {
		t.Fatalf("summary = %+v", document.Summary)
	}
	if document.ExitCode != 2 {
		t.Fatalf("exit code = %d", document.ExitCode)
	}
}

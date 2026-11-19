package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shellghostt/envdoctor/internal/bundle"
	"github.com/shellghostt/envdoctor/internal/cli"
	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/shellghostt/envdoctor/internal/report"
)

type panicCollector struct{}

func (panicCollector) Name() string { return "path" }
func (panicCollector) Collect(context.Context) (collector.Part, error) {
	panic("offline diagnosis ran a collector")
}

func TestM4CLIFormatAndTerminalMatrix(t *testing.T) {
	capable := report.TerminalState{Input: true, Output: true, Error: true, Term: "xterm-256color"}
	for _, tc := range []struct {
		name        string
		flags       []string
		term        report.TerminalState
		interactive bool
		kind        string
	}{
		{"default-viewer", nil, capable, true, "text"},
		{"no-tty", nil, report.TerminalState{}, false, "text"},
		{"stdout-redirect", nil, report.TerminalState{Input: true, Error: true}, false, "text"},
		{"stderr-redirect", nil, report.TerminalState{Input: true, Output: true}, false, "text"},
		{"stdin-redirect", nil, report.TerminalState{Output: true, Error: true}, false, "text"},
		{"dumb", nil, report.TerminalState{Input: true, Output: true, Error: true, Term: "dumb"}, false, "text"},
		{"empty-no-color", nil, report.TerminalState{Input: true, Output: true, Error: true, NoColor: true}, false, "text"},
		{"windows-unset-term", nil, report.TerminalState{Input: true, Output: true, Error: true}, true, "text"},
		{"plain", []string{"--plain"}, capable, false, "text"},
		{"text", []string{"--format", "text"}, capable, false, "text"},
		{"json", []string{"--format", "json"}, capable, false, "json"},
		{"json-alias", []string{"--json"}, capable, false, "json"},
		{"compatible-json", []string{"--json", "--format", "json"}, capable, false, "json"},
		{"markdown", []string{"--format", "markdown"}, capable, false, "markdown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			app := newApp(&out, &errOut)
			app.Engine = diagnostic.NewEngine(warnRule{severity: diagnostic.SeverityWarning})
			app.Terminals = func() report.TerminalState { return tc.term }
			called := false
			app.Interactive = func(_ context.Context, _ io.Reader, w io.Writer, d report.Document) error {
				called = true
				return report.WriteText(w, d)
			}
			code := cli.Main(append([]string{"envdoctor", "diagnose", "--verbose"}, tc.flags...), app)
			if code != 1 || called != tc.interactive {
				t.Fatalf("exit=%d viewer=%v stderr=%s", code, called, errOut.String())
			}
			if strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "collector platform") {
				t.Fatal("stdout contaminated")
			}
			if !strings.Contains(errOut.String(), "collector platform") {
				t.Fatal("verbose diagnostics missing")
			}
			switch tc.kind {
			case "json":
				validateAgainstSchema(t, compileDiagnoseSchema(t), out.Bytes())
			case "markdown":
				if !strings.HasPrefix(out.String(), "# EnvDoctor diagnosis\n") || !strings.Contains(out.String(), "ENV003") {
					t.Fatal("Markdown report missing")
				}
			default:
				if !strings.Contains(out.String(), "Broken PATH entry") {
					t.Fatal("plain report missing")
				}
			}
		})
	}
}

func TestM4CLIInvalidFormatsBeforeCollection(t *testing.T) {
	for _, flags := range [][]string{{"--format", "html"}, {"--format", ""}, {"--json", "--format", "text"}, {"--json", "--format", "markdown"}, {"--plain", "--json"}, {"--plain", "--format", "markdown"}} {
		var out, errOut bytes.Buffer
		app := newApp(&out, &errOut)
		app.Collectors = []collector.Collector{panicCollector{}}
		app.Terminals = func() report.TerminalState { panic("format conflict inspected a terminal") }
		if code := cli.Main(append([]string{"envdoctor", "diagnose"}, flags...), app); code != 4 || out.Len() != 0 {
			t.Fatalf("exit=%d stdout=%s", code, out.String())
		}
	}
}

func TestM4CLIInteractiveFailurePreservesFallbackAndExit(t *testing.T) {
	var out, errOut bytes.Buffer
	app := newApp(&out, &errOut)
	app.Terminals = func() report.TerminalState { return report.TerminalState{Input: true, Output: true, Error: true} }
	app.Interactive = func(_ context.Context, _ io.Reader, w io.Writer, d report.Document) error {
		if err := report.WriteText(w, d); err != nil {
			return err
		}
		return context.Canceled
	}
	if code := cli.Main([]string{"envdoctor", "diagnose"}, app); code != 3 || !strings.Contains(out.String(), "environment diagnosis") {
		t.Fatalf("exit=%d output=%s", code, out.String())
	}
}

func offlineSnapshot() collector.Snapshot {
	name, email := "SyntheticOfflineUser", "synthetic@private.invalid"
	return collector.Snapshot{SchemaVersion: 1, CollectedAt: fixedNow,
		Platform: &collector.Platform{OS: "windows", Version: "11", Arch: "amd64"}, Shell: &collector.Shell{Name: "powershell"},
		Path:         &collector.PathInfo{Entries: []collector.PathEntry{{Raw: `C:\Users\SyntheticOfflineUser\gone`, Position: 0}}},
		Git:          &collector.Git{Version: "2.53.0", UserName: &name, UserEmail: &email, IdentityChecked: true},
		Dependencies: &collector.Dependencies{ProjectRoot: `C:\Users\SyntheticOfflineUser\project`},
	}
}

func bundleFile(t *testing.T, snapshot collector.Snapshot) string {
	t.Helper()
	findings, err := diagnostic.DefaultEngine().Evaluate(snapshot, diagnostic.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	doc := report.NewDocument("0.1.0", fixedNow, snapshot, findings, nil, 12)
	data, err := bundle.Build(context.Background(), doc, bundle.Options{IncludeLogs: true})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "capture.zip")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func offlineApp(out, errOut *bytes.Buffer) cli.App {
	app := newApp(out, errOut)
	app.Version = "0.2.0"
	app.Engine = diagnostic.DefaultEngine()
	app.Collectors = []collector.Collector{panicCollector{}}
	app.HomeDir = func() (string, error) { panic("offline diagnosis inspected the home directory") }
	app.Getenv = func(string) string { panic("explicit offline format read the environment") }
	app.LookupEnv = func(string) (string, bool) { panic("explicit offline format read the environment") }
	return app
}

func TestM4CLIOfflineEvidenceAndProvenance(t *testing.T) {
	path := bundleFile(t, offlineSnapshot())
	for _, flags := range [][]string{{"--json"}, {"--format", "markdown"}, {"--plain"}, {"--format", "text"}, {"--json", "--rule", "ENV003", "--verbose"}, {"--json", "--component", "docker"}} {
		var out, errOut bytes.Buffer
		code := cli.Main(append([]string{"envdoctor", "diagnose", "--from-bundle", path}, flags...), offlineApp(&out, &errOut))
		want := 1
		if strings.Contains(strings.Join(flags, " "), "--component docker") {
			want = 0
		}
		if code != want {
			t.Fatalf("exit=%d stderr=%s", code, errOut.String())
		}
		if strings.Contains(out.String(), "SyntheticOfflineUser") || strings.Contains(out.String(), "synthetic@private.invalid") {
			t.Fatal("original identity leaked")
		}
		if flags[0] == "--json" {
			var doc report.Document
			if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			if doc.Source == nil || doc.Source.CaptureTool.Version != "0.1.0" || doc.Tool.Version != "0.2.0" || !doc.Snapshot.CollectedAt.Equal(fixedNow) {
				t.Fatal("capture/evaluator provenance missing")
			}
			validateAgainstSchema(t, compileDiagnoseSchema(t), out.Bytes())
		}
	}
}

func TestM4CLIOfflineAllExitCodes(t *testing.T) {
	for code := 0; code <= 3; code++ {
		s := offlineSnapshot()
		s.Path.Entries[0].Exists = true
		s.Path.Entries[0].IsDir = true
		switch code {
		case 1:
			s.Path.Entries[0].Exists = false
			s.Path.Entries[0].IsDir = false
		case 2:
			s.Docker = &collector.Docker{Installed: true, CLIVersion: "29.9.2", ProbeStatus: "unavailable"}
		case 3:
			s.Errors = []collector.CollectorError{{Collector: "runtimes", Message: "secret failure"}}
		}
		var out, errOut bytes.Buffer
		if got := cli.Main([]string{"envdoctor", "diagnose", "--from-bundle", bundleFile(t, s), "--json"}, offlineApp(&out, &errOut)); got != code {
			t.Fatalf("got=%d want=%d stderr=%s", got, code, errOut.String())
		}
	}
}

func TestM4CLIConflictsAndReadFailuresAreOffline(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "SECRET-PATH-not-present.zip")
	for _, extra := range [][]string{{"--fixture-dir", "SECRET-PATH"}, {"--port", "3000"}, {"--probe-network"}, {"--probe-network=false"}, {"--package-manager", "npm"}, {"--fixture-dir="}} {
		var out, errOut bytes.Buffer
		code := cli.Main(append([]string{"envdoctor", "diagnose", "--from-bundle", missing, "--json"}, extra...), offlineApp(&out, &errOut))
		if code != 4 || out.Len() != 0 || strings.Contains(errOut.String(), "SECRET-PATH") {
			t.Fatalf("conflict exit=%d stderr=%s", code, errOut.String())
		}
	}
	for _, path := range []string{missing, filepath.Join(t.TempDir(), "SECRET-PATH-invalid.zip")} {
		if path != missing {
			if err := os.WriteFile(path, []byte("SECRET-CAPTURED-ERROR"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		var out, errOut bytes.Buffer
		if code := cli.Main([]string{"envdoctor", "diagnose", "--from-bundle", path, "--json"}, offlineApp(&out, &errOut)); code != 3 || out.Len() != 0 || strings.Contains(errOut.String(), "SECRET-") {
			t.Fatalf("invalid input exit=%d stderr=%s", code, errOut.String())
		}
	}
}

func TestM4CLIBundleWriterSuccessAndFailures(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture")
	if err := os.Mkdir(fixture, 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(offlineSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture, "snapshot.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "SECRET-OUTPUT.zip")
	var out, errOut bytes.Buffer
	app := offlineApp(&out, &errOut)
	code := cli.Main([]string{"envdoctor", "bundle", "--fixture-dir", fixture, "-o", destination, "--include-logs"}, app)
	if code != 0 || errOut.Len() != 0 || !strings.Contains(out.String(), "original diagnosis exit 1") || strings.Contains(out.String(), "SECRET-") {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	if _, err := bundle.Load(context.Background(), destination); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{"--output", ""}, {"--json"}, {"--rule", "ENV003"}, {"--from-bundle", destination}, {"--fixture-dir", fixture, "--probe-network"}} {
		out.Reset()
		errOut.Reset()
		if code := cli.Main(append([]string{"envdoctor", "bundle"}, flags...), offlineApp(&out, &errOut)); code != 4 {
			t.Fatalf("usage exit=%d stderr=%s", code, errOut.String())
		}
	}
	out.Reset()
	errOut.Reset()
	if code := cli.Main([]string{"envdoctor", "bundle", "--fixture-dir", fixture, "--output", dir}, offlineApp(&out, &errOut)); code != 3 {
		t.Fatalf("I/O exit=%d stderr=%s", code, errOut.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out.Reset()
	errOut.Reset()
	app = offlineApp(&out, &errOut)
	app.Context = ctx
	if code := cli.Main([]string{"envdoctor", "bundle", "--fixture-dir", fixture, "--output", destination}, app); code != 3 {
		t.Fatalf("cancel exit=%d", code)
	}
}

type m4WriteFailure struct{}

func (m4WriteFailure) Write([]byte) (int, error) { return 0, errors.New("SECRET-WRITER-ERROR") }

type m4WriteShort struct{}

func (m4WriteShort) Write(p []byte) (int, error) { return max(0, len(p)-1), nil }

func TestM4CLIBundleStatusFailurePreservesDestination(t *testing.T) {
	for _, writer := range []struct {
		name   string
		output io.Writer
	}{{"failure", m4WriteFailure{}}, {"short-write", m4WriteShort{}}} {
		t.Run(writer.name, func(t *testing.T) {
			dir := t.TempDir()
			fixture := filepath.Join(dir, "fixture")
			if err := os.Mkdir(fixture, 0700); err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(offlineSnapshot())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(fixture, "snapshot.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(dir, "previous.zip")
			previous := []byte("previous capture must survive a failed status write")
			if err := os.WriteFile(destination, previous, 0600); err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			app := offlineApp(&out, &errOut)
			app.Stdout = writer.output
			if code := cli.Main([]string{"envdoctor", "bundle", "--fixture-dir", fixture, "--output", destination}, app); code != 3 || strings.Contains(errOut.String(), "SECRET-") {
				t.Fatalf("exit=%d stderr=%s", code, errOut.String())
			}
			got, err := os.ReadFile(destination)
			if err != nil || !bytes.Equal(got, previous) {
				t.Fatalf("previous destination changed: %q, %v", got, err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 2 {
				t.Fatalf("owned staging file leaked: %v, %v", entries, err)
			}
		})
	}
}

func TestM4CLIStructuredOutputErrors(t *testing.T) {
	path := bundleFile(t, offlineSnapshot())
	for _, format := range []string{"text", "markdown", "json"} {
		var out, errOut bytes.Buffer
		app := offlineApp(&out, &errOut)
		app.Stdout = m4WriteFailure{}
		if code := cli.Main([]string{"envdoctor", "diagnose", "--from-bundle", path, "--format", format}, app); code != 3 || strings.Contains(errOut.String(), "SECRET-") {
			t.Fatalf("exit=%d stderr=%s", code, errOut.String())
		}
	}
}

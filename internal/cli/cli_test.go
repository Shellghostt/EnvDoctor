package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shellghostt/envdoctor/internal/cli"
	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
)

var (
	fixedNow  = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	fixedHome = `C:\Users\ann`
)

// staticCollector returns a prepared part so that CLI tests do not depend on
// the machine running them.
type staticCollector struct {
	name string
	part collector.Part
	err  error
}

func (c staticCollector) Name() string { return c.name }

func (c staticCollector) Collect(context.Context) (collector.Part, error) {
	return c.part, c.err
}

func healthyCollectors() []collector.Collector {
	return []collector.Collector{
		staticCollector{name: "platform", part: collector.Part{Platform: &collector.Platform{
			OS: "windows", Version: "11 (build 22631)", Arch: "amd64",
		}}},
		staticCollector{name: "shell", part: collector.Part{Shell: &collector.Shell{Name: "powershell"}}},
		staticCollector{name: "path", part: collector.Part{Path: &collector.PathInfo{Entries: []collector.PathEntry{
			{Raw: `C:\Users\ann\bin`, Exists: true, IsDir: true, Position: 0},
			{Raw: `C:\Users\ann\gone`, Exists: false, Position: 1},
			{Raw: `C:\Users\ann\bin`, Exists: true, IsDir: true, Position: 2, Duplicate: true},
		}}}},
	}
}

func newApp(stdout, stderr *bytes.Buffer) cli.App {
	return cli.App{
		Version:    "0.1.0",
		Stdout:     stdout,
		Stderr:     stderr,
		Getenv:     func(string) string { return "" },
		Now:        func() time.Time { return fixedNow },
		HomeDir:    func() (string, error) { return fixedHome, nil },
		Collectors: healthyCollectors(),
		Engine:     diagnostic.NewEngine(), // Keep the M1 renderer contract test independent of rules.
	}
}

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errOut bytes.Buffer
	code = cli.Main(append([]string{"envdoctor"}, args...), newApp(&out, &errOut))
	return code, out.String(), errOut.String()
}

func TestVersionCommand(t *testing.T) {
	code, stdout, stderr := run(t, "version")

	if code != cli.ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	if stdout != "envdoctor 0.1.0\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestVersionFlag(t *testing.T) {
	code, stdout, _ := run(t, "--version")

	if code != cli.ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout, "0.1.0") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestDiagnoseText(t *testing.T) {
	code, stdout, stderr := run(t, "diagnose")

	if code != cli.ExitOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{
		"Platform   windows 11 (build 22631) amd64",
		"Shell      powershell",
		"PATH       3 entries, 1 missing, 1 duplicate",
		"  - ~\\gone",
		"Rules      0 registered in this build",
		"No findings.",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, fixedHome) {
		t.Error("home directory was not redacted from stdout")
	}
	if stderr != "" {
		t.Errorf("stderr should be empty without --verbose: %q", stderr)
	}
}

func TestDiagnoseJSONOnlyWritesOneDocument(t *testing.T) {
	code, stdout, stderr := run(t, "diagnose", "--json")

	if code != cli.ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}

	var document map[string]any
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if document["rules"] != float64(0) {
		t.Errorf("rules = %v", document["rules"])
	}
	if document["exitCode"] != float64(0) {
		t.Errorf("exitCode = %v", document["exitCode"])
	}
	if strings.Contains(stdout, fixedHome) {
		t.Error("home directory was not redacted from JSON")
	}
}

func TestDiagnoseVerboseWritesTimingsToStderr(t *testing.T) {
	code, stdout, stderr := run(t, "diagnose", "--verbose")

	if code != cli.ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr, "collector platform") {
		t.Fatalf("timings missing from stderr: %q", stderr)
	}
	if strings.Contains(stdout, "collector platform") {
		t.Fatal("timings leaked into stdout")
	}
}

func TestUnknownFlagExitsWithUsage(t *testing.T) {
	code, stdout, stderr := run(t, "diagnose", "--nope")

	if code != cli.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, cli.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "unknown flag") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestUnknownRuleExitsWithUsage(t *testing.T) {
	code, _, stderr := run(t, "diagnose", "--rule", "ENV999")

	if code != cli.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, cli.ExitUsage)
	}
	if !strings.Contains(stderr, "ENV999") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestMalformedRuleIDIsRejectedBeforeRunning(t *testing.T) {
	code, _, stderr := run(t, "diagnose", "--rule", "ENV1")

	if code != cli.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, cli.ExitUsage)
	}
	if !strings.Contains(stderr, "ENV001") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestCollectorFailureExitsThree(t *testing.T) {
	var out, errOut bytes.Buffer
	app := newApp(&out, &errOut)
	app.Collectors = append(healthyCollectors(), staticCollector{
		name: "broken",
		err:  errors.New("permission denied"),
	})

	code := cli.Main([]string{"envdoctor", "diagnose"}, app)

	if code != cli.ExitToolFailure {
		t.Fatalf("exit code = %d, want %d", code, cli.ExitToolFailure)
	}
	if !strings.Contains(out.String(), "Degraded collectors") {
		t.Fatalf("degraded notice missing:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "broken: permission denied") {
		t.Fatalf("collector error missing:\n%s", out.String())
	}
}

type warnRule struct{ severity diagnostic.Severity }

func (r warnRule) ID() string           { return "ENV003" }
func (r warnRule) Components() []string { return []string{"path"} }
func (r warnRule) Evaluate(collector.Snapshot) []diagnostic.Finding {
	return []diagnostic.Finding{{
		RuleID:    "ENV003",
		Severity:  r.severity,
		Component: "path",
		Title:     "Broken PATH entry",
		Detected:  []string{`~\gone does not exist`},
	}}
}

func TestFindingsDriveExitCode(t *testing.T) {
	cases := []struct {
		severity diagnostic.Severity
		want     int
	}{
		{diagnostic.SeverityInfo, cli.ExitOK},
		{diagnostic.SeverityWarning, cli.ExitWarning},
		{diagnostic.SeverityError, cli.ExitError},
		{diagnostic.SeverityCritical, cli.ExitError},
	}

	for _, tc := range cases {
		t.Run(string(tc.severity), func(t *testing.T) {
			var out, errOut bytes.Buffer
			app := newApp(&out, &errOut)
			app.Engine = diagnostic.NewEngine(warnRule{severity: tc.severity})

			code := cli.Main([]string{"envdoctor", "diagnose"}, app)

			if code != tc.want {
				t.Fatalf("exit code = %d, want %d", code, tc.want)
			}
			if !strings.Contains(out.String(), "Broken PATH entry") {
				t.Fatalf("finding not rendered:\n%s", out.String())
			}
		})
	}
}

func TestComponentFilter(t *testing.T) {
	var out, errOut bytes.Buffer
	app := newApp(&out, &errOut)
	app.Engine = diagnostic.NewEngine(warnRule{severity: diagnostic.SeverityWarning})

	code := cli.Main([]string{"envdoctor", "diagnose", "--component", "docker"}, app)
	if code != cli.ExitOK {
		t.Fatalf("exit code = %d, want 0 when the filter excludes every rule", code)
	}
	if strings.Contains(out.String(), "Broken PATH entry") {
		t.Fatal("finding from another component was rendered")
	}
}

package cli_test

import (
	"bytes"
	"encoding/json"
	"github.com/shellghostt/envdoctor/internal/cli"
	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultRulesAndSchema(t *testing.T) {
	var out, errs bytes.Buffer
	app := newApp(&out, &errs)
	app.Engine = nil
	code := cli.Main([]string{"envdoctor", "diagnose", "--json"}, app)
	if code != 1 {
		t.Fatalf("exit=%d %s", code, errs.String())
	}
	validateAgainstSchema(t, compileDiagnoseSchema(t), out.Bytes())
	if !strings.Contains(out.String(), "ENV003") {
		t.Fatal("default rule did not run")
	}
}
func TestFixtureCLIIsOfflineAndExitCodesMatch(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "tests", "golden", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			f, err := diagnostic.LoadFixture(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			raw, _ := json.Marshal(f.Snapshot)
			if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			var out, errs bytes.Buffer
			app := newApp(&out, &errs)
			app.Engine = nil
			app.Collectors = []collector.Collector{staticCollector{err: os.ErrPermission}}
			code := cli.Main([]string{"envdoctor", "diagnose", "--fixture-dir", dir, "--rule", f.RuleID, "--json"}, app)
			wantCode := diagnostic.ExitCode(f.ExpectedFindings)
			if len(f.Snapshot.Errors) > 0 {
				wantCode = cli.ExitToolFailure
			}
			if code != wantCode {
				t.Fatalf("exit=%d stderr=%s", code, errs.String())
			}
			validateAgainstSchema(t, compileDiagnoseSchema(t), out.Bytes())
		})
	}
}

func TestOfflineInputRejectsTrailingDocuments(t *testing.T) {
	dir := t.TempDir()
	raw, err := json.Marshal(collector.Snapshot{SchemaVersion: 1, CollectedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "snapshot.json"), append(raw, []byte(` {"schemaVersion":999}`)...), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := cli.Main([]string{"envdoctor", "diagnose", "--fixture-dir", dir, "--json"}, newApp(&out, &errs)); code != cli.ExitToolFailure {
		t.Fatalf("trailing document was accepted: exit=%d output=%s", code, out.String())
	}
}

func TestJSONSchemaAllowsFailedPlatformCollector(t *testing.T) {
	var out, errs bytes.Buffer
	app := newApp(&out, &errs)
	app.Collectors = []collector.Collector{staticCollector{name: "platform", err: os.ErrPermission}}
	if code := cli.Main([]string{"envdoctor", "diagnose", "--json"}, app); code != cli.ExitToolFailure {
		t.Fatalf("failed platform exit=%d", code)
	}
	validateAgainstSchema(t, compileDiagnoseSchema(t), out.Bytes())
}

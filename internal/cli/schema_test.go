package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/shellghostt/envdoctor/internal/cli"
	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
)

const (
	diagnoseSchemaID = "https://github.com/shellghostt/envdoctor/schema/diagnose-v1.schema.json"
	snapshotSchemaID = "https://github.com/shellghostt/envdoctor/schema/snapshot-v1.schema.json"
)

// compileDiagnoseSchema loads the two committed schema files and resolves the
// snapshot reference by $id rather than by filesystem path, so the test behaves
// the same on Windows, Linux and macOS.
func compileDiagnoseSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()

	dir := filepath.Join("..", "..", "docs", "schema")
	snapshotBytes, err := os.ReadFile(filepath.Join(dir, "snapshot-v1.schema.json"))
	if err != nil {
		t.Fatalf("read snapshot schema: %v", err)
	}
	diagnoseBytes, err := os.ReadFile(filepath.Join(dir, "diagnose-v1.schema.json"))
	if err != nil {
		t.Fatalf("read diagnose schema: %v", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(snapshotSchemaID, bytes.NewReader(snapshotBytes)); err != nil {
		t.Fatalf("add snapshot schema: %v", err)
	}
	if err := compiler.AddResource(diagnoseSchemaID, bytes.NewReader(diagnoseBytes)); err != nil {
		t.Fatalf("add diagnose schema: %v", err)
	}

	schema, err := compiler.Compile(diagnoseSchemaID)
	if err != nil {
		t.Fatalf("compile diagnose schema: %v", err)
	}
	return schema
}

func validateAgainstSchema(t *testing.T, schema *jsonschema.Schema, raw []byte) {
	t.Helper()

	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatalf("output does not match docs/schema/diagnose-v1.schema.json: %v\n%s", err, raw)
	}
}

func TestJSONOutputMatchesSchema(t *testing.T) {
	schema := compileDiagnoseSchema(t)

	var out, errOut bytes.Buffer
	code := cli.Main([]string{"envdoctor", "diagnose", "--json"}, newApp(&out, &errOut))
	if code != cli.ExitOK {
		t.Fatalf("exit code = %d, stderr = %q", code, errOut.String())
	}

	validateAgainstSchema(t, schema, out.Bytes())
}

// schemaRule exists to put a finding with every optional field through the
// schema, not to test a real diagnosis.
type schemaRule struct{}

func (schemaRule) ID() string           { return "ENV006" }
func (schemaRule) Components() []string { return []string{"docker"} }
func (schemaRule) Evaluate(collector.Snapshot) []diagnostic.Finding {
	return []diagnostic.Finding{{
		RuleID:    "ENV006",
		Severity:  diagnostic.SeverityError,
		Component: "docker",
		Title:     "Docker daemon unavailable",
		Detected:  []string{"Docker CLI installed (29.9.2)", "Docker daemon not responding"},
		Cause:     "Docker Desktop is not running.",
		Fix: &diagnostic.Fix{
			Summary:           "Start Docker Desktop and retry.",
			Commands:          []string{"Start-Process 'Docker Desktop.exe'"},
			Automatic:         true,
			RequiresElevation: false,
			Reversible:        true,
		},
		Verify: "envdoctor diagnose --component docker",
		Docs:   "https://github.com/shellghostt/envdoctor/blob/main/docs/rules/ENV006.md",
	}}
}

func TestJSONOutputWithFindingsMatchesSchema(t *testing.T) {
	schema := compileDiagnoseSchema(t)

	var out, errOut bytes.Buffer
	app := newApp(&out, &errOut)
	app.Engine = diagnostic.NewEngine(schemaRule{})

	code := cli.Main([]string{"envdoctor", "diagnose", "--json"}, app)
	if code != cli.ExitError {
		t.Fatalf("exit code = %d, want %d", code, cli.ExitError)
	}

	validateAgainstSchema(t, schema, out.Bytes())
}

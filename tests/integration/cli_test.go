package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/shellghostt/envdoctor/internal/report"
)

func TestBuiltCLIWithAllGoldenFixtures(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "envdoctor")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/envdoctor")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, out)
	}
	compiler := jsonschema.NewCompiler()
	for _, name := range []string{"snapshot", "diagnose"} {
		path := filepath.Join(root, "docs", "schema", name+"-v1.schema.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = compiler.AddResource("https://github.com/shellghostt/envdoctor/schema/"+name+"-v1.schema.json", bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
	}
	schema, err := compiler.Compile("https://github.com/shellghostt/envdoctor/schema/diagnose-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	fixtures, err := filepath.Glob(filepath.Join(root, "tests", "golden", "*.json"))
	if err != nil || len(fixtures) != 54 {
		t.Fatalf("expected 54 golden cases: %d %v", len(fixtures), err)
	}
	for _, path := range fixtures {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			fixture, err := diagnostic.LoadFixture(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			snapshot, err := json.Marshal(fixture.Snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, "snapshot.json"), snapshot, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "diagnose", "--fixture-dir", dir, "--rule", fixture.RuleID, "--json")
			var out, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &stderr
			err = cmd.Run()
			code := 0
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					code = exit.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			wantCode := diagnostic.ExitCode(fixture.ExpectedFindings)
			if len(fixture.Snapshot.Errors) > 0 {
				wantCode = 3
			}
			if code != wantCode || stderr.Len() != 0 {
				t.Fatalf("exit=%d want=%d stderr=%s", code, wantCode, stderr.String())
			}
			var document report.Document
			decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
			if err = decoder.Decode(&document); err != nil {
				t.Fatal(err)
			}
			if err = decoder.Decode(new(any)); err != io.EOF {
				t.Fatal("CLI wrote more than one JSON document")
			}
			if len(document.Findings) == 0 {
				document.Findings = nil
			}
			if len(fixture.ExpectedFindings) == 0 {
				fixture.ExpectedFindings = nil
			}
			if document.ExitCode != code || document.Rules != 6 || !reflect.DeepEqual(document.Findings, fixture.ExpectedFindings) {
				t.Fatalf("unexpected CLI report: %s", out.String())
			}
			var decoded any
			if err = json.Unmarshal(out.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}
			if err = schema.Validate(decoded); err != nil {
				t.Fatalf("report schema: %v", err)
			}
		})
	}
}

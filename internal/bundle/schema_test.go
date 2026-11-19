package bundle_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/shellghostt/envdoctor/internal/bundle"
	"github.com/shellghostt/envdoctor/internal/report"
)

func compileBundleSchemas(t *testing.T) map[string]*jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.LoadURL = func(string) (io.ReadCloser, error) {
		return nil, fmt.Errorf("unregistered schema; network loading forbidden")
	}
	dir := filepath.Join("..", "..", "docs", "schema")
	files, err := filepath.Glob(filepath.Join(dir, "*.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var id struct {
			ID string `json:"$id"`
		}
		if err := json.Unmarshal(data, &id); err != nil {
			t.Fatal(err)
		}
		if err := compiler.AddResource(id.ID, bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
	}
	result := map[string]*jsonschema.Schema{}
	for member, name := range map[string]string{"manifest.json": "bundle-manifest-v1", "snapshot.json": "bundle-snapshot-v1", "report.json": "bundle-report-v1", "versions.json": "bundle-versions-v1", "diagnostics.json": "bundle-diagnostics-v1"} {
		schema, err := compiler.Compile("https://github.com/shellghostt/envdoctor/schema/" + name + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		result[member] = schema
	}
	return result
}

func TestM4BundleMembersMatchCommittedSchemas(t *testing.T) {
	schemas := compileBundleSchemas(t)
	unsupported := sensitiveDocument()
	unsupported.Snapshot.Runtimes[0].Version = "SECRET-VERSION"
	for _, doc := range []report.Document{sensitiveDocument(), unsupported} {
		data, err := bundle.Build(context.Background(), doc, bundle.Options{IncludeLogs: true})
		if err != nil {
			t.Fatal(err)
		}
		for name, body := range members(t, data) {
			if name == "report.md" {
				continue
			}
			var value any
			if err := json.Unmarshal(body, &value); err != nil {
				t.Fatal(err)
			}
			if err := schemas[name].Validate(value); err != nil {
				t.Fatalf("%s does not match committed schema: %v", name, err)
			}
		}
	}
}

func TestM4OptionalEmptyAliasesMatchSchemaAndReader(t *testing.T) {
	doc := sensitiveDocument()
	for i := range doc.Snapshot.Runtimes {
		doc.Snapshot.Runtimes[i].ResolvedPath = ""
		doc.Snapshot.Runtimes[i].EnvironmentRoot = ""
	}
	doc.Snapshot.Dependencies.Requirements[0].Path = ""
	doc.Snapshot.PythonEnvironment.CanonicalRoot = ""
	doc.Snapshot.PythonEnvironment.MetadataKnown = false
	data, err := bundle.Build(context.Background(), doc, bundle.Options{})
	if err != nil {
		t.Fatal(err)
	}
	all := members(t, data)
	var snapshot map[string]any
	if err := json.Unmarshal(all["snapshot.json"], &snapshot); err != nil {
		t.Fatal(err)
	}
	for _, value := range snapshot["runtimes"].([]any) {
		runtime := value.(map[string]any)
		runtime["resolvedToken"], runtime["environmentToken"] = "", ""
	}
	requirement := snapshot["dependencies"].(map[string]any)["requirements"].([]any)[0].(map[string]any)
	requirement["pathToken"] = ""
	snapshot["pythonEnvironment"].(map[string]any)["canonicalToken"] = ""
	schema := compileBundleSchemas(t)["snapshot.json"]
	if err := schema.Validate(snapshot); err != nil {
		t.Fatalf("empty optional aliases disagree with schema: %v", err)
	}
	all["snapshot.json"], err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.Read(context.Background(), repack(t, all, zip.Deflate)); err != nil {
		t.Fatalf("empty optional aliases disagree with reader: %v", err)
	}
	for _, invalid := range []string{"C:/Users/SECRET-IDENTITY", "path-0000", "path-4097"} {
		requirement["pathToken"] = invalid
		if err := schema.Validate(snapshot); err == nil {
			t.Fatalf("schema accepted invalid alias %q", invalid)
		}
		all["snapshot.json"], err = json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bundle.Read(context.Background(), repack(t, all, zip.Deflate)); err == nil {
			t.Fatalf("reader accepted invalid alias %q", invalid)
		}
	}
}

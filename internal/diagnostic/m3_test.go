package diagnostic_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
)

func TestRegistryPagesAndFixturesCoverEveryBuiltin(t *testing.T) {
	seen := map[string]bool{}
	metadata := diagnostic.BuiltinMetadata()
	if len(metadata) != 12 {
		t.Fatalf("registry size=%d", len(metadata))
	}
	for _, rule := range metadata {
		if seen[rule.RuleID] || rule.Title == "" || rule.Evidence == "" || len(rule.ManualSteps) == 0 || len(rule.Caveats) == 0 || len(rule.Prerequisites) == 0 {
			t.Fatalf("incomplete/duplicate rule: %+v", rule)
		}
		seen[rule.RuleID] = true
		page, err := os.ReadFile(filepath.Join("..", "..", "docs", "rules", rule.RuleID+".md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(page) != diagnostic.RuleMarkdown(rule) {
			t.Fatalf("generated page drift: %s; run make docs", rule.RuleID)
		}
		for _, goos := range []string{"windows", "linux", "darwin"} {
			for _, kind := range []string{"positive", "negative", "edge"} {
				path := filepath.Join("..", "..", "tests", "golden", fmt.Sprintf("%s-%s-%s.json", rule.RuleID, goos, kind))
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("missing rule/OS/case fixture: %v", err)
				}
				fixture, err := diagnostic.LoadFixture(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				if fixture.RuleID != rule.RuleID || fixture.Snapshot.Platform == nil || fixture.Snapshot.Platform.OS != goos {
					t.Fatalf("fixture evidence is for a different rule/OS: %s", path)
				}
			}
		}
	}
	if !reflect.DeepEqual(diagnostic.DefaultEngine().Rules(), func() []string {
		var ids []string
		for _, m := range metadata {
			ids = append(ids, m.RuleID)
		}
		return ids
	}()) {
		t.Fatal("engine and metadata registry disagree")
	}
}

func TestMetadataCopiesCannotMutateRulesOrDocumentation(t *testing.T) {
	before, _ := diagnostic.LookupRule("ENV007")
	metadata := diagnostic.BuiltinMetadata()
	metadata[6].Title = "corrupted"
	metadata[6].Prerequisites[0] = "corrupted"
	metadata[6].ManualSteps[0] = "corrupted"
	metadata[6].Caveats[0] = "corrupted"
	lookup, _ := diagnostic.LookupRule("ENV007")
	lookup.Prerequisites[0] = "also corrupted"
	after, _ := diagnostic.LookupRule("ENV007")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("public metadata mutated the builtin registry")
	}
}

func TestFailedCollectorPrerequisitesDisableEveryM3Rule(t *testing.T) {
	for _, rule := range diagnostic.BuiltinMetadata()[6:] {
		data, err := os.ReadFile(filepath.Join("..", "..", "tests", "golden", rule.RuleID+"-linux-positive.json"))
		if err != nil {
			t.Fatal(err)
		}
		fixture, err := diagnostic.LoadFixture(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range rule.Prerequisites {
			fixture.Snapshot.Errors = []collector.CollectorError{{Collector: name, Message: "permission denied"}}
			got, err := diagnostic.EvaluateFixture(diagnostic.DefaultEngine(), fixture)
			if err != nil || len(got) != 0 {
				t.Fatalf("%s used failed %s evidence: %+v %v", rule.RuleID, name, got, err)
			}
		}
	}
}

func TestUnknownResolutionDoesNotCreateM2Findings(t *testing.T) {
	for _, goos := range []string{"windows", "linux", "darwin"} {
		cases := map[string][]collector.Runtime{
			"ENV001": {{Name: "python", Version: "3.11.0", Path: "opaque/python", Active: true, Resolution: "unknown"}, {Name: "python", Version: "3.12.0", Path: "known/python", Active: true, Resolution: "known"}},
			"ENV002": {{Name: "node", Path: "opaque/node", Resolution: "unknown"}, {Name: "node", Path: "known/node", Active: true, Resolution: "known"}},
			"ENV004": {{Name: "python", Path: "opaque/python", Expected: true, Resolution: "unknown"}, {Name: "python", Path: "known/python", Active: true, Resolution: "known"}},
		}
		for id, runtimes := range cases {
			snapshot := collector.Snapshot{Platform: &collector.Platform{OS: goos, Arch: "amd64"}, Runtimes: runtimes}
			got, err := diagnostic.DefaultEngine().Evaluate(snapshot, diagnostic.Filter{RuleID: id})
			if err != nil || len(got) != 0 {
				t.Fatalf("%s %s guessed from opaque resolution: %+v %v", goos, id, got, err)
			}
		}
	}
}

func BenchmarkM3RuleEvaluation(b *testing.B) {
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "golden", "ENV009-linux-positive.json"))
	if err != nil {
		b.Fatal(err)
	}
	fixture, err := diagnostic.LoadFixture(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}
	engine := diagnostic.DefaultEngine()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findings, err := engine.Evaluate(fixture.Snapshot, diagnostic.Filter{})
		if err != nil || len(findings) != 1 {
			b.Fatalf("findings=%+v err=%v", findings, err)
		}
	}
}

func TestM3RuleEvaluationIsDeterministicAndDoesNotMutateSnapshot(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "tests", "golden", "ENV0*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fixture, err := diagnostic.LoadFixture(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(fixture.Snapshot)
		first, err := diagnostic.EvaluateFixture(diagnostic.DefaultEngine(), fixture)
		if err != nil {
			t.Fatal(err)
		}
		second, err := diagnostic.EvaluateFixture(diagnostic.DefaultEngine(), fixture)
		if err != nil {
			t.Fatal(err)
		}
		after, _ := json.Marshal(fixture.Snapshot)
		if !bytes.Equal(before, after) || !reflect.DeepEqual(first, second) {
			t.Fatalf("rule changed its snapshot or varied across runs: %s", path)
		}
	}
}

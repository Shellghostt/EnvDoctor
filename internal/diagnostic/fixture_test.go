package diagnostic_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shellghostt/envdoctor/internal/diagnostic"
)

func TestLoadFixtureRejectsUnknownFields(t *testing.T) {
	body := `{"ruleId":"ENV003","snapshot":{},"expectedFindings":[],"surprise":true}`

	if _, err := diagnostic.LoadFixture(strings.NewReader(body)); err == nil {
		t.Fatal("unknown fixture field was accepted")
	}
}

func TestLoadFixtureRequiresRuleID(t *testing.T) {
	if _, err := diagnostic.LoadFixture(strings.NewReader(`{"snapshot":{},"expectedFindings":[]}`)); err == nil {
		t.Fatal("fixture without ruleId was accepted")
	}
}

func TestEvaluateFixtureUsesOnlyItsRule(t *testing.T) {
	engine := diagnostic.NewEngine(
		testRule{id: "ENV003", components: []string{"path"}, findings: []diagnostic.Finding{
			finding("ENV003", "path", diagnostic.SeverityWarning),
		}},
		testRule{id: "ENV001", components: []string{"python"}, findings: []diagnostic.Finding{
			finding("ENV001", "python", diagnostic.SeverityError),
		}},
	)

	fixture := diagnostic.Fixture{
		RuleID:           "ENV003",
		ExpectedFindings: []diagnostic.Finding{finding("ENV003", "path", diagnostic.SeverityWarning)},
	}

	got, err := diagnostic.EvaluateFixture(engine, fixture)
	if err != nil {
		t.Fatalf("evaluate fixture: %v", err)
	}
	if len(got) != 1 || got[0].RuleID != "ENV003" {
		t.Fatalf("findings = %+v", got)
	}
}

func TestEvaluateFixtureRejectsUnknownRule(t *testing.T) {
	engine := diagnostic.NewEngine()
	if _, err := diagnostic.EvaluateFixture(engine, diagnostic.Fixture{RuleID: "ENV042"}); err == nil {
		t.Fatal("a fixture for a missing rule must fail")
	}
}

// TestGoldenFixtures is the harness required by PLAN ENV-007. Every file in
// tests/golden is a fixture; evaluating it must reproduce the expected findings
// exactly. There are no fixtures while there are no rules, and a fixture for a
// rule that does not exist fails.
func TestGoldenFixtures(t *testing.T) {
	dir := filepath.Join("..", "..", "tests", "golden")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read golden dir: %v", err)
	}

	engine := diagnostic.NewEngine()
	seen := map[string]bool{}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			file, err := os.Open(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatalf("open fixture: %v", err)
			}
			defer func() { _ = file.Close() }()

			fixture, err := diagnostic.LoadFixture(file)
			if err != nil {
				t.Fatalf("load fixture: %v", err)
			}
			seen[fixture.RuleID] = true

			got, err := diagnostic.EvaluateFixture(engine, fixture)
			if err != nil {
				t.Fatalf("evaluate fixture: %v", err)
			}
			if !reflect.DeepEqual(normalize(got), normalize(fixture.ExpectedFindings)) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				wantJSON, _ := json.MarshalIndent(fixture.ExpectedFindings, "", "  ")
				t.Fatalf("findings differ\n got: %s\nwant: %s", gotJSON, wantJSON)
			}
		})
	}

	for _, id := range engine.Rules() {
		if !seen[id] {
			t.Errorf("rule %s has no fixture in tests/golden", id)
		}
	}
}

func normalize(findings []diagnostic.Finding) []diagnostic.Finding {
	if findings == nil {
		return []diagnostic.Finding{}
	}
	return findings
}

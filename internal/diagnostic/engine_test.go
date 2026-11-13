package diagnostic_test

import (
	"testing"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
)

type testRule struct {
	id         string
	components []string
	findings   []diagnostic.Finding
}

func (r testRule) ID() string           { return r.id }
func (r testRule) Components() []string { return r.components }
func (r testRule) Evaluate(collector.Snapshot) []diagnostic.Finding {
	return r.findings
}

func finding(id, title string, severity diagnostic.Severity) diagnostic.Finding {
	return diagnostic.Finding{
		RuleID:    id,
		Severity:  severity,
		Component: "path",
		Title:     title,
		Detected:  []string{"evidence"},
	}
}

func TestEngineWithNoRulesReturnsEmptySlice(t *testing.T) {
	findings, err := diagnostic.NewEngine().Evaluate(collector.Snapshot{}, diagnostic.Filter{})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if findings == nil {
		t.Fatal("findings must be an empty slice, not nil, so JSON renders []")
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestEngineCollectsAndSorts(t *testing.T) {
	engine := diagnostic.NewEngine(
		testRule{id: "ENV003", components: []string{"path"}, findings: []diagnostic.Finding{
			finding("ENV003", "warning", diagnostic.SeverityWarning),
		}},
		testRule{id: "ENV001", components: []string{"python"}, findings: []diagnostic.Finding{
			finding("ENV001", "error", diagnostic.SeverityError),
		}},
	)

	findings, err := engine.Evaluate(collector.Snapshot{}, diagnostic.Filter{})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %+v", findings)
	}
	if findings[0].RuleID != "ENV001" {
		t.Fatalf("severity order not applied: %+v", findings)
	}
}

func TestEngineFiltersByComponentAndRule(t *testing.T) {
	engine := diagnostic.NewEngine(
		testRule{id: "ENV001", components: []string{"python"}, findings: []diagnostic.Finding{
			finding("ENV001", "python", diagnostic.SeverityError),
		}},
		testRule{id: "ENV003", components: []string{"path"}, findings: []diagnostic.Finding{
			finding("ENV003", "path", diagnostic.SeverityWarning),
		}},
	)

	byComponent, err := engine.Evaluate(collector.Snapshot{}, diagnostic.Filter{Component: "path"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(byComponent) != 1 || byComponent[0].RuleID != "ENV003" {
		t.Fatalf("component filter = %+v", byComponent)
	}

	byRule, err := engine.Evaluate(collector.Snapshot{}, diagnostic.Filter{RuleID: "ENV001"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(byRule) != 1 || byRule[0].RuleID != "ENV001" {
		t.Fatalf("rule filter = %+v", byRule)
	}
}

func TestEngineRejectsFindingWithoutEvidence(t *testing.T) {
	bad := finding("ENV003", "no evidence", diagnostic.SeverityWarning)
	bad.Detected = nil

	engine := diagnostic.NewEngine(testRule{id: "ENV003", components: []string{"path"}, findings: []diagnostic.Finding{bad}})

	if _, err := engine.Evaluate(collector.Snapshot{}, diagnostic.Filter{}); err == nil {
		t.Fatal("invalid finding must fail the run")
	}
}

func TestEngineRejectsFindingClaimingAnotherRule(t *testing.T) {
	engine := diagnostic.NewEngine(testRule{id: "ENV003", components: []string{"path"}, findings: []diagnostic.Finding{
		finding("ENV004", "borrowed", diagnostic.SeverityWarning),
	}})

	if _, err := engine.Evaluate(collector.Snapshot{}, diagnostic.Filter{}); err == nil {
		t.Fatal("a rule must not emit findings under another rule id")
	}
}

func TestEngineFillsRuleIDWhenOmitted(t *testing.T) {
	unnamed := finding("ENV003", "title", diagnostic.SeverityWarning)
	unnamed.RuleID = ""

	engine := diagnostic.NewEngine(testRule{id: "ENV003", components: []string{"path"}, findings: []diagnostic.Finding{unnamed}})

	findings, err := engine.Evaluate(collector.Snapshot{}, diagnostic.Filter{})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if findings[0].RuleID != "ENV003" {
		t.Fatalf("rule id = %q", findings[0].RuleID)
	}
}

func TestEngineHasRule(t *testing.T) {
	engine := diagnostic.NewEngine(testRule{id: "ENV006"})

	if !engine.HasRule("ENV006") {
		t.Error("registered rule not found")
	}
	if engine.HasRule("ENV999") {
		t.Error("unknown rule reported as present")
	}
	if got := engine.Rules(); len(got) != 1 || got[0] != "ENV006" {
		t.Errorf("rules = %v", got)
	}
}

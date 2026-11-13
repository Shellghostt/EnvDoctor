package diagnostic

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/shellghostt/envdoctor/internal/collector"
)

// Fixture is one golden test case: a captured snapshot and the exact findings
// the rule must produce. Fixtures live in tests/golden/<rule id>.json.
type Fixture struct {
	RuleID           string             `json:"ruleId"`
	Description      string             `json:"description,omitempty"`
	Snapshot         collector.Snapshot `json:"snapshot"`
	ExpectedFindings []Finding          `json:"expectedFindings"`
}

// LoadFixture decodes one fixture. Unknown fields are rejected so that a typo
// in a golden file fails instead of being ignored.
func LoadFixture(r io.Reader) (Fixture, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	var fixture Fixture
	if err := dec.Decode(&fixture); err != nil {
		return Fixture{}, fmt.Errorf("decode fixture: %w", err)
	}
	if fixture.RuleID == "" {
		return Fixture{}, fmt.Errorf("fixture has no ruleId")
	}
	return fixture, nil
}

// EvaluateFixture runs the engine against a fixture's snapshot and returns the
// findings for the fixture's rule. A fixture that names an unknown rule fails,
// because a golden file for a rule that does not exist is stale documentation.
func EvaluateFixture(engine *Engine, fixture Fixture) ([]Finding, error) {
	if !engine.HasRule(fixture.RuleID) {
		return nil, fmt.Errorf("fixture refers to rule %s, which is not registered in this build", fixture.RuleID)
	}
	// A fixture may contain findings for its own rule only. Filtering keeps the
	// comparison focused even when the snapshot would trigger other rules.
	return engine.Evaluate(fixture.Snapshot, Filter{RuleID: fixture.RuleID})
}

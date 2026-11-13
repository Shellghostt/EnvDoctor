package diagnostic

import (
	"fmt"

	"github.com/shellghostt/envdoctor/internal/collector"
)

// Rule is a pure function from a snapshot to zero or more findings.
//
// It must not read the environment, execute anything, or depend on anything
// outside the snapshot it is given.
type Rule interface {
	ID() string
	Components() []string
	Evaluate(snapshot collector.Snapshot) []Finding
}

// Filter narrows the findings after evaluation. Empty fields mean no filter.
type Filter struct {
	Component string
	RuleID    string
}

// Engine evaluates the registered rules in order.
type Engine struct {
	rules []Rule
}

// NewEngine builds an engine. The registry is fixed for the life of the
// process; there is no dynamic loading and no plugin path.
func NewEngine(rules ...Rule) *Engine {
	return &Engine{rules: rules}
}

// Rules returns the registered rule ids in registration order.
func (e *Engine) Rules() []string {
	ids := make([]string, 0, len(e.rules))
	for _, rule := range e.rules {
		ids = append(ids, rule.ID())
	}
	return ids
}

// HasRule reports whether a rule id is registered in this build.
func (e *Engine) HasRule(id string) bool {
	for _, rule := range e.rules {
		if rule.ID() == id {
			return true
		}
	}
	return false
}

// Evaluate runs rules whose components match the filter and returns validated,
// sorted findings. A rule that produces an invalid finding is a bug, so an
// error is returned instead of the finding.
func (e *Engine) Evaluate(snapshot collector.Snapshot, filter Filter) ([]Finding, error) {
	findings := make([]Finding, 0, 8)

	for _, rule := range e.rules {
		if filter.RuleID != "" && rule.ID() != filter.RuleID {
			continue
		}
		if filter.Component != "" && !matchesComponent(rule, filter.Component) {
			continue
		}

		for _, finding := range rule.Evaluate(snapshot) {
			if finding.RuleID == "" {
				finding.RuleID = rule.ID()
			}
			if finding.RuleID != rule.ID() {
				return nil, fmt.Errorf("rule %s produced a finding for %s", rule.ID(), finding.RuleID)
			}
			if err := finding.Validate(); err != nil {
				return nil, err
			}
			findings = append(findings, finding)
		}
	}

	Sort(findings)
	return findings, nil
}

func matchesComponent(rule Rule, component string) bool {
	for _, candidate := range rule.Components() {
		if candidate == component {
			return true
		}
	}
	return false
}

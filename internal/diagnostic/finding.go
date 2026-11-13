// Package diagnostic turns a snapshot into findings. It is the only place in
// EnvDoctor where judgement happens, and it never touches the operating system.
package diagnostic

import (
	"fmt"
	"regexp"
	"sort"
)

// Severity is the weight of a finding. The order matters: it decides both the
// report order and the process exit code.
type Severity string

// Severities, lowest to highest.
const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Rank returns a comparable weight. Unknown severities rank below info so that
// an invalid finding is never silently promoted.
func (s Severity) Rank() int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityError:
		return 2
	case SeverityWarning:
		return 1
	case SeverityInfo:
		return 0
	default:
		return -1
	}
}

// Valid reports whether the severity is one of the four defined values.
func (s Severity) Valid() bool { return s.Rank() >= 0 }

// Fix is a proposed remediation. It is data only: the fix engine, not a rule,
// decides whether anything is executed.
type Fix struct {
	Summary           string   `json:"summary"`
	Commands          []string `json:"commands,omitempty"`
	Automatic         bool     `json:"automatic"`
	RequiresElevation bool     `json:"requiresElevation"`
	Reversible        bool     `json:"reversible"`
}

// Finding is one diagnosis. Detected must never be empty: an explanation with
// no evidence is a guess.
type Finding struct {
	RuleID    string   `json:"ruleId"`
	Severity  Severity `json:"severity"`
	Component string   `json:"component"`
	Title     string   `json:"title"`
	Detected  []string `json:"detected"`
	Cause     string   `json:"cause,omitempty"`
	Fix       *Fix     `json:"fix,omitempty"`
	Verify    string   `json:"verify,omitempty"`
	Docs      string   `json:"docs,omitempty"`
}

var ruleIDPattern = regexp.MustCompile(`^ENV[0-9]{3}$`)

// Validate checks the invariants from PLAN section 9. It is called by the
// engine, so a broken rule fails the run instead of printing a plausible lie.
func (f Finding) Validate() error {
	if !ruleIDPattern.MatchString(f.RuleID) {
		return fmt.Errorf("finding has invalid rule id %q", f.RuleID)
	}
	if !f.Severity.Valid() {
		return fmt.Errorf("finding %s has invalid severity %q", f.RuleID, f.Severity)
	}
	if f.Component == "" {
		return fmt.Errorf("finding %s has no component", f.RuleID)
	}
	if f.Title == "" {
		return fmt.Errorf("finding %s has no title", f.RuleID)
	}
	if len(f.Detected) == 0 {
		return fmt.Errorf("finding %s has no evidence in detected", f.RuleID)
	}
	return nil
}

// ExitCode maps findings to the documented process exit code: 2 for anything at
// error or above, 1 for warnings, 0 otherwise.
func ExitCode(findings []Finding) int {
	code := 0
	for _, f := range findings {
		switch {
		case f.Severity.Rank() >= SeverityError.Rank():
			return 2
		case f.Severity == SeverityWarning:
			code = 1
		}
	}
	return code
}

// Sort orders findings by severity, highest first, then by rule id and title so
// that output is deterministic.
func Sort(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity.Rank() != findings[j].Severity.Rank() {
			return findings[i].Severity.Rank() > findings[j].Severity.Rank()
		}
		if findings[i].RuleID != findings[j].RuleID {
			return findings[i].RuleID < findings[j].RuleID
		}
		return findings[i].Title < findings[j].Title
	})
}

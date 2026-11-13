package diagnostic_test

import (
	"testing"

	"github.com/shellghostt/envdoctor/internal/diagnostic"
)

func validFinding() diagnostic.Finding {
	return diagnostic.Finding{
		RuleID:    "ENV003",
		Severity:  diagnostic.SeverityWarning,
		Component: "path",
		Title:     "Broken PATH entry",
		Detected:  []string{"C:\\gone does not exist"},
	}
}

func TestValidateAcceptsMinimalFinding(t *testing.T) {
	if err := validFinding().Validate(); err != nil {
		t.Fatalf("valid finding rejected: %v", err)
	}
}

func TestValidateRejectsMissingEvidence(t *testing.T) {
	finding := validFinding()
	finding.Detected = nil

	if err := finding.Validate(); err == nil {
		t.Fatal("a finding with no evidence must be rejected")
	}
}

func TestValidateRejectsBadRuleID(t *testing.T) {
	finding := validFinding()
	finding.RuleID = "ENV3"

	if err := finding.Validate(); err == nil {
		t.Fatal("malformed rule id accepted")
	}
}

func TestValidateRejectsUnknownSeverity(t *testing.T) {
	finding := validFinding()
	finding.Severity = "fatal"

	if err := finding.Validate(); err == nil {
		t.Fatal("unknown severity accepted")
	}
}

func TestSeverityRankIsOrdered(t *testing.T) {
	order := []diagnostic.Severity{
		diagnostic.SeverityInfo,
		diagnostic.SeverityWarning,
		diagnostic.SeverityError,
		diagnostic.SeverityCritical,
	}
	for i := 1; i < len(order); i++ {
		if order[i-1].Rank() >= order[i].Rank() {
			t.Fatalf("%s should rank below %s", order[i-1], order[i])
		}
	}
}

func TestExitCode(t *testing.T) {
	none := []diagnostic.Finding{}
	info := []diagnostic.Finding{{Severity: diagnostic.SeverityInfo}}
	warning := []diagnostic.Finding{{Severity: diagnostic.SeverityWarning}}
	errorFinding := []diagnostic.Finding{{Severity: diagnostic.SeverityError}}
	critical := []diagnostic.Finding{{Severity: diagnostic.SeverityCritical}}

	cases := []struct {
		name     string
		findings []diagnostic.Finding
		want     int
	}{
		{"none", none, 0},
		{"info", info, 0},
		{"warning", warning, 1},
		{"warning and info", append(warning, info...), 1},
		{"error", errorFinding, 2},
		{"error beats warning", append(errorFinding, warning...), 2},
		{"critical", critical, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := diagnostic.ExitCode(tc.findings); got != tc.want {
				t.Fatalf("exit code = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestSortIsSeverityFirstThenStable(t *testing.T) {
	findings := []diagnostic.Finding{
		{RuleID: "ENV010", Severity: diagnostic.SeverityWarning, Title: "b"},
		{RuleID: "ENV003", Severity: diagnostic.SeverityError, Title: "a"},
		{RuleID: "ENV004", Severity: diagnostic.SeverityWarning, Title: "a"},
	}

	diagnostic.Sort(findings)

	want := []string{"ENV003", "ENV004", "ENV010"}
	for i, id := range want {
		if findings[i].RuleID != id {
			t.Fatalf("order = %v, want %v", findings, want)
		}
	}
}

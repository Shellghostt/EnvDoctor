// Package report renders a diagnosis. It adds no facts: everything printed
// comes from the snapshot or the findings.
package report

import (
	"time"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
)

// Document is the complete result of `envdoctor diagnose`. The JSON form is
// versioned and described by docs/schema/diagnose-v1.schema.json.
type Document struct {
	SchemaVersion int                  `json:"schemaVersion"`
	GeneratedAt   time.Time            `json:"generatedAt"`
	Tool          Tool                 `json:"tool"`
	Snapshot      collector.Snapshot   `json:"snapshot"`
	Findings      []diagnostic.Finding `json:"findings"`
	Summary       Summary              `json:"summary"`
	ExitCode      int                  `json:"exitCode"`
	Rules         int                  `json:"rules"`
	Collectors    []Collector          `json:"collectors"`
}

// Tool identifies the binary that produced the document.
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Collector is one collector's outcome.
type Collector struct {
	Name       string `json:"name"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

// Summary counts findings by severity.
type Summary struct {
	Total    int `json:"total"`
	Info     int `json:"info"`
	Warning  int `json:"warning"`
	Error    int `json:"error"`
	Critical int `json:"critical"`
}

// NewDocument assembles a document. Findings are copied so that the caller
// cannot mutate the document afterwards.
func NewDocument(version string, generatedAt time.Time, snapshot collector.Snapshot, findings []diagnostic.Finding, timings []collector.Timing, ruleCount int) Document {
	if findings == nil {
		findings = []diagnostic.Finding{}
	}
	copied := make([]diagnostic.Finding, len(findings))
	copy(copied, findings)

	collectors := make([]Collector, 0, len(timings))
	summary := Summary{Total: len(copied)}
	for _, finding := range copied {
		switch finding.Severity {
		case diagnostic.SeverityInfo:
			summary.Info++
		case diagnostic.SeverityWarning:
			summary.Warning++
		case diagnostic.SeverityError:
			summary.Error++
		case diagnostic.SeverityCritical:
			summary.Critical++
		}
	}
	for _, timing := range timings {
		entry := Collector{
			Name:       timing.Name,
			DurationMs: timing.Duration.Milliseconds(),
		}
		if timing.Err != nil {
			entry.Error = timing.Err.Error()
		}
		collectors = append(collectors, entry)
	}
	// Offline snapshots have errors but no live timing records. Preserve their
	// degraded status and expose the same notices as a live diagnosis.
	for _, failure := range snapshot.Errors {
		matched := false
		for i := range collectors {
			if collectors[i].Name == failure.Collector {
				collectors[i].Error = failure.Message
				matched = true
				break
			}
		}
		if !matched {
			collectors = append(collectors, Collector{Name: failure.Collector, Error: failure.Message})
		}
	}
	code := exitCode(timings, copied)
	if len(snapshot.Errors) > 0 {
		code = 3
	}

	return Document{
		SchemaVersion: 1,
		GeneratedAt:   generatedAt.UTC(),
		Tool:          Tool{Name: "envdoctor", Version: version},
		Snapshot:      snapshot,
		Findings:      copied,
		Summary:       summary,
		ExitCode:      code,
		Rules:         ruleCount,
		Collectors:    collectors,
	}
}

// exitCode applies the documented mapping, with one exception: a collector
// failure means the report is incomplete, which is a tool failure (3) even if
// the findings that did run would have exited 0.
func exitCode(timings []collector.Timing, findings []diagnostic.Finding) int {
	if collector.Degraded(timings) {
		return 3
	}
	return diagnostic.ExitCode(findings)
}

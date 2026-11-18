package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/shellghostt/envdoctor/internal/diagnostic"
)

const MarkdownMaxBytes = 2 << 20

type markdownBuffer struct {
	buffer strings.Builder
	err    error
}

func (b *markdownBuffer) raw(s string) {
	if b.err != nil {
		return
	}
	if len(s) > MarkdownMaxBytes-b.buffer.Len() {
		b.err = fmt.Errorf("markdown report exceeds 2 MiB")
		return
	}
	b.buffer.WriteString(s)
}

// Escape all dynamic punctuation: a renderer cannot turn captured URLs,
// brackets, HTML, backticks or fence sequences into active Markdown content.
func (b *markdownBuffer) text(s string) {
	for _, r := range s {
		if b.err != nil {
			return
		}
		if r == ' ' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.raw(string(r))
		} else if displayControl(r) {
			b.raw("&#92;u" + fmt.Sprintf("%04X", r))
		} else {
			b.raw("&#" + strconv.Itoa(int(r)) + ";")
		}
	}
}

func (b *markdownBuffer) line(prefix, value string) {
	b.raw(prefix)
	b.text(value)
	b.raw("\n")
}

func WriteMarkdown(w io.Writer, doc Document) error {
	b := &markdownBuffer{}
	b.raw("# EnvDoctor diagnosis\n\n")
	b.line("- Tool: ", doc.Tool.Name+" "+doc.Tool.Version)
	b.line("- Evaluated: ", doc.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))
	b.line("- Captured: ", doc.Snapshot.CollectedAt.Format("2006-01-02T15:04:05Z07:00"))
	if p := doc.Snapshot.Platform; p != nil {
		parts := []string{p.OS}
		if p.Version != "" {
			parts = append(parts, p.Version)
		}
		parts = append(parts, p.Arch)
		b.line("- Platform: ", strings.Join(parts, " "))
	} else {
		b.raw("- Platform: unavailable\n")
	}
	b.raw(fmt.Sprintf("- Exit code: %d\n- Findings: %d (critical %d, error %d, warning %d, info %d)\n\n",
		doc.ExitCode, doc.Summary.Total, doc.Summary.Critical, doc.Summary.Error, doc.Summary.Warning, doc.Summary.Info))
	if source := doc.Source; source != nil {
		b.raw("## Captured evidence\n\n")
		b.line("- Source: ", source.Kind)
		b.line("- Capture tool: ", source.CaptureTool.Name+" "+source.CaptureTool.Version)
		b.line("- Redaction policy: ", source.RedactionPolicy)
		b.raw(fmt.Sprintf("- Original diagnosis exit: %d\n\n", source.CaptureExitCode))
		b.raw("Paths and hosts are opaque capture-local aliases. Removed values remain unavailable.\n\n")
		for _, omission := range source.Omissions {
			b.line("- Omitted: ", omission.Field+" ("+omission.Reason+")")
		}
		b.raw("\n")
	}
	b.raw("## Degraded collectors\n\n")
	failed := map[string]bool{}
	for _, c := range doc.Collectors {
		if c.Error != "" {
			b.line("- ", c.Name+": "+c.Error)
			failed[c.Name] = true
		}
	}
	if len(failed) == 0 {
		b.raw("None recorded.\n")
	}
	b.raw("\n## Skipped builtin rules\n\n")
	skipped := false
	for _, rule := range diagnostic.BuiltinMetadata() {
		for _, prerequisite := range rule.Prerequisites {
			if failed[prerequisite] {
				b.line("- ", rule.RuleID+": unavailable "+prerequisite+" evidence")
				skipped = true
				break
			}
		}
	}
	if !skipped {
		b.raw("None gated by recorded collector failures.\n")
	}
	b.raw("\n## Findings\n\n")
	if len(doc.Findings) == 0 {
		b.raw("No findings. Unavailable evidence remains inconclusive.\n")
	}
	for _, f := range doc.Findings {
		b.line("### ", strings.ToUpper(string(f.Severity))+" — "+f.RuleID+" — "+f.Title)
		b.raw("\n")
		for _, evidence := range f.Detected {
			b.line("- Detected: ", evidence)
		}
		if f.Cause != "" {
			b.line("- Possible cause: ", f.Cause)
		}
		if f.Fix != nil {
			b.line("- Manual suggestion: ", f.Fix.Summary)
			for _, command := range f.Fix.Commands {
				b.line("- Manual command (not executed): ", command)
			}
		}
		if f.Verify != "" {
			b.line("- Verification (not executed): ", f.Verify)
		}
		if metadata, ok := diagnostic.LookupRule(f.RuleID); ok {
			for _, step := range metadata.ManualSteps {
				if f.Fix == nil || step != f.Fix.Summary {
					b.line("- Manual suggestion: ", step)
				}
			}
			for _, caveat := range metadata.Caveats {
				b.line("- Limitation: ", caveat)
			}
		} else {
			b.raw("- Limitation: Rule metadata unavailable in this build.\n")
		}
		if f.Docs != "" {
			b.line("- Rule documentation: ", f.Docs)
		}
		b.raw("\n")
	}
	if b.err != nil {
		return b.err
	}
	if err := writeString(w, strings.TrimRight(b.buffer.String(), "\n")+"\n"); err != nil {
		return fmt.Errorf("write Markdown report: %w", err)
	}
	return nil
}

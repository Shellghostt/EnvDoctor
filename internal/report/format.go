package report

import (
	"fmt"
	"strings"
	"unicode"
)

type Mode string

const (
	Text        Mode = "text"
	JSON        Mode = "json"
	Markdown    Mode = "markdown"
	Interactive Mode = "interactive"
)

type FormatOptions struct {
	Format   string
	Explicit bool
	JSON     bool
	Plain    bool
}

// TerminalState is supplied by the CLI. Selection itself never inspects a host.
type TerminalState struct {
	Input, Output, Error bool
	Term                 string
	NoColor              bool
}

func SelectMode(o FormatOptions, terminal TerminalState) (Mode, error) {
	if o.Explicit && o.Format != string(Text) && o.Format != string(JSON) && o.Format != string(Markdown) {
		return "", fmt.Errorf("--format must be text, markdown or json")
	}
	if o.JSON && o.Explicit && o.Format != string(JSON) {
		return "", fmt.Errorf("--json conflicts with --format")
	}
	if o.Plain && (o.JSON || o.Explicit && o.Format != string(Text)) {
		return "", fmt.Errorf("--plain requires text output")
	}
	if o.JSON {
		return JSON, nil
	}
	if o.Explicit {
		return Mode(o.Format), nil
	}
	if o.Plain || terminal.NoColor || strings.EqualFold(strings.TrimSpace(terminal.Term), "dumb") || !terminal.Input || !terminal.Output || !terminal.Error {
		return Text, nil
	}
	return Interactive, nil
}

// VisibleText preserves ordinary report bytes and makes terminal controls data.
func VisibleText(s string) string {
	if !strings.ContainsFunc(s, func(r rune) bool { return r != '\n' && r != '\t' && displayControl(r) }) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r != '\n' && r != '\t' && displayControl(r) {
			fmt.Fprintf(&b, "\\u%04X", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func displayControl(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }

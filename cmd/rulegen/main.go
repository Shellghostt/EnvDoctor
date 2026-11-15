// Command rulegen writes/checks rule pages from the same registry as the CLI.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shellghostt/envdoctor/internal/diagnostic"
)

func main() {
	check := flag.Bool("check", false, "check committed pages without writing")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "rulegen accepts only --check")
		os.Exit(1)
	}
	if !*check {
		if err := os.MkdirAll(filepath.Join("docs", "rules"), 0755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	failed := false
	for _, rule := range diagnostic.BuiltinMetadata() {
		path := filepath.Join("docs", "rules", rule.RuleID+".md")
		expected := diagnostic.RuleMarkdown(rule)
		if *check {
			actual, err := os.ReadFile(path)
			if err != nil || string(actual) != expected {
				fmt.Fprintf(os.Stderr, "%s differs; run make docs\n", path)
				failed = true
			}
		} else if err := os.WriteFile(path, []byte(expected), 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

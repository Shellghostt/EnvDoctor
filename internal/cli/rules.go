package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/spf13/cobra"
)

func (a *App) newExplainCommand() *cobra.Command {
	return &cobra.Command{
		Use: "explain <rule-id>", Short: "Print a rule's full documentation without inspecting this machine", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			metadata, ok := diagnostic.LookupRule(args[0])
			if !ok {
				return fmt.Errorf("rule %s is not registered in this build", args[0])
			}
			if _, err := io.WriteString(a.Stdout, diagnostic.RuleMarkdown(metadata)); err != nil {
				return toolFailure{err}
			}
			return nil
		},
	}
}

func (a *App) newRulesCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use: "rules", Short: "List builtin rule metadata without inspecting this machine", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			metadata := diagnostic.BuiltinMetadata()
			if asJSON {
				encoder := json.NewEncoder(a.Stdout)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(diagnostic.RegistryDocument{SchemaVersion: 1, Rules: metadata}); err != nil {
					return toolFailure{err}
				}
				return nil
			}
			for _, rule := range metadata {
				if _, err := fmt.Fprintf(a.Stdout, "%s [%s] %s: %s\n", rule.RuleID, rule.Severity, rule.Component, rule.Title); err != nil {
					return toolFailure{err}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "write one versioned registry JSON document")
	return cmd
}

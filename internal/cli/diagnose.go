package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/config"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/shellghostt/envdoctor/internal/report"
)

func (a *App) newDiagnoseCommand() *cobra.Command {
	opts := config.Options{}

	cmd := &cobra.Command{
		Use:   "diagnose",
		Short: "Collect environment facts and report findings",
		Long: "Collects facts about this machine and reports findings from the rules in this build.\n" +
			"With --json, writes exactly one JSON document to stdout and nothing else.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.Validate(); err != nil {
				return err
			}
			if opts.Rule != "" && !a.Engine.HasRule(opts.Rule) {
				return fmt.Errorf("rule %s is not registered in this build", opts.Rule)
			}

			document, err := a.diagnose(cmd.Context(), opts)
			if err != nil {
				return toolFailure{err}
			}

			if opts.JSON {
				if err := report.WriteJSON(a.Stdout, document); err != nil {
					return toolFailure{err}
				}
			} else if err := report.WriteText(a.Stdout, document); err != nil {
				return toolFailure{err}
			}

			if opts.Verbose {
				writeTimings(a, document)
			}

			a.exitCode = document.ExitCode
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "write one JSON document to stdout")
	cmd.Flags().BoolVar(&opts.Verbose, "verbose", false, "write collector timings to stderr")
	cmd.Flags().StringVar(&opts.Component, "component", "", "only evaluate rules for this component")
	cmd.Flags().StringVar(&opts.Rule, "rule", "", "only evaluate this rule id (ENV001)")

	return cmd
}

func (a *App) diagnose(ctx context.Context, opts config.Options) (report.Document, error) {
	runner := collector.Runner{
		Collectors: a.Collectors,
		Timeout:    config.CollectorTimeout,
		Now:        a.Now,
	}
	snapshot, timings := runner.Run(ctx)

	if home, err := a.HomeDir(); err == nil {
		snapshot.Redact(home)
	}

	findings, err := a.Engine.Evaluate(snapshot, diagnostic.Filter{
		Component: opts.Component,
		RuleID:    opts.Rule,
	})
	if err != nil {
		return report.Document{}, fmt.Errorf("evaluate rules: %w", err)
	}

	return report.NewDocument(a.Version, a.Now(), snapshot, findings, timings, len(a.Engine.Rules())), nil
}

func writeTimings(a *App, document report.Document) {
	for _, c := range document.Collectors {
		status := "ok"
		if c.Error != "" {
			status = c.Error
		}
		_, _ = fmt.Fprintf(a.Stderr, "collector %-10s %5dms  %s\n", c.Name, c.DurationMs, status)
	}
}

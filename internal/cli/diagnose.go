package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/shellghostt/envdoctor/internal/bundle"
	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/config"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/shellghostt/envdoctor/internal/report"
)

func (a *App) newDiagnoseCommand() *cobra.Command {
	opts := config.Options{}
	format := report.FormatOptions{}
	cmd := &cobra.Command{
		Use: "diagnose", Short: "Collect environment facts and report findings",
		Long: "Report environment facts and manual advice as text, Markdown or JSON.\n" +
			"Capable terminals use a scrollable viewer by default; --plain forces text.\n" +
			"--from-bundle evaluates captured facts offline without inspecting this machine.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format.JSON, format.Explicit = opts.JSON, cmd.Flags().Changed("format")
			if err := opts.Validate(); err != nil {
				return err
			}
			if cmd.Flags().Changed("from-bundle") {
				if opts.FromBundle == "" {
					return fmt.Errorf("--from-bundle requires a file")
				}
				for _, flag := range []string{"fixture-dir", "port", "probe-network", "package-manager"} {
					if cmd.Flags().Changed(flag) {
						return fmt.Errorf("--from-bundle conflicts with live capture flags and --fixture-dir")
					}
				}
			}
			// Validate formats before terminal lookup or evidence I/O.
			mode, err := report.SelectMode(format, report.TerminalState{})
			if err != nil {
				return err
			}
			if !format.JSON && !format.Explicit && !format.Plain {
				mode, err = report.SelectMode(format, a.terminalState())
				if err != nil {
					return err
				}
			}
			if opts.Rule != "" && !a.Engine.HasRule(opts.Rule) {
				return fmt.Errorf("rule %s is not registered in this build", opts.Rule)
			}
			document, err := a.diagnose(cmd.Context(), opts)
			if err != nil {
				return toolFailure{err}
			}
			switch mode {
			case report.JSON:
				err = report.WriteJSON(a.Stdout, document)
			case report.Markdown:
				err = report.WriteMarkdown(a.Stdout, document)
			case report.Interactive:
				err = a.Interactive(cmd.Context(), a.Stdin, a.Stdout, document)
			default:
				err = report.WriteText(a.Stdout, document)
			}
			if err != nil {
				return toolFailure{err}
			}
			if opts.Verbose {
				if err := writeTimings(a, document); err != nil {
					return toolFailure{err}
				}
			}
			a.exitCode = document.ExitCode
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "write one JSON document to stdout")
	cmd.Flags().StringVar(&format.Format, "format", "", "output format: text, markdown or json")
	cmd.Flags().BoolVar(&format.Plain, "plain", false, "force plain text without the terminal viewer")
	cmd.Flags().BoolVar(&opts.Verbose, "verbose", false, "write collector timings to stderr")
	cmd.Flags().StringVar(&opts.Component, "component", "", "only evaluate rules for this component")
	cmd.Flags().StringVar(&opts.Rule, "rule", "", "only evaluate this rule id (ENV001)")
	cmd.Flags().StringVar(&opts.FromBundle, "from-bundle", "", "evaluate a sharing bundle offline; run no collectors")
	bindCaptureFlags(cmd, &opts)
	return cmd
}

func bindCaptureFlags(cmd *cobra.Command, opts *config.Options) {
	cmd.Flags().IntSliceVar(&opts.Ports, "port", nil, "TCP ports your application needs (repeatable or comma separated)")
	cmd.Flags().StringVar(&opts.FixtureDir, "fixture-dir", "", "read snapshot.json in this directory; do not inspect this machine")
	cmd.Flags().BoolVar(&opts.ProbeNetwork, "probe-network", false, "probe configured proxy endpoints with a one-second total deadline")
	cmd.Flags().StringVar(&opts.PackageManager, "package-manager", "", "explicit project runner: npm, pnpm, yarn or bun, optionally @major.minor.patch")
}

func (a *App) diagnose(ctx context.Context, opts config.Options) (report.Document, error) {
	if opts.FromBundle != "" {
		capture, err := bundle.Load(ctx, opts.FromBundle)
		if err != nil {
			return report.Document{}, err
		}
		findings, err := a.Engine.Evaluate(capture.Snapshot, diagnostic.Filter{Component: opts.Component, RuleID: opts.Rule})
		if err != nil {
			return report.Document{}, fmt.Errorf("evaluate rules: %w", err)
		}
		doc := report.NewDocument(a.Version, a.Now(), capture.Snapshot, findings, nil, len(a.Engine.Rules()))
		doc.Source = &capture.Source
		return doc, nil
	}
	return a.captureDocument(ctx, opts, true)
}

func (a *App) captureDocument(ctx context.Context, opts config.Options, redact bool) (report.Document, error) {
	var snapshot collector.Snapshot
	var timings []collector.Timing
	if opts.FixtureDir != "" {
		loaded, err := loadSnapshot(opts.FixtureDir)
		if err != nil {
			return report.Document{}, err
		}
		snapshot = loaded
		if len(opts.Ports) > 0 {
			snapshot.RequestedPorts = opts.Ports
		}
	} else {
		collectors := append([]collector.Collector(nil), a.Collectors...)
		for i, c := range collectors {
			switch value := c.(type) {
			case collector.PortCollector:
				value.Requested = opts.Ports
				collectors[i] = value
			case collector.ProxyCollector:
				value.ProbeNetwork = opts.ProbeNetwork
				collectors[i] = value
			case collector.DependencyCollector:
				value.PackageManager = opts.PackageManager
				collectors[i] = value
			}
		}
		runner := collector.Runner{Collectors: collectors, Timeout: config.CollectorTimeout, Now: a.Now}
		snapshot, timings = runner.Run(ctx)
		snapshot.RequestedPorts = opts.Ports
	}
	if err := ctx.Err(); err != nil {
		return report.Document{}, err
	}
	// Sharing export must see exact facts before home redaction can collapse
	// distinct paths. Ordinary reports retain their existing M3 redaction.
	if redact {
		home, _ := a.HomeDir()
		snapshot.Redact(home)
		for i := range timings {
			if timings[i].Err != nil {
				timings[i].Err = errors.New(collector.RedactText(timings[i].Err.Error(), home))
			}
		}
	}
	findings, err := a.Engine.Evaluate(snapshot, diagnostic.Filter{Component: opts.Component, RuleID: opts.Rule})
	if err != nil {
		return report.Document{}, fmt.Errorf("evaluate rules: %w", err)
	}
	return report.NewDocument(a.Version, a.Now(), snapshot, findings, timings, len(a.Engine.Rules())), nil
}

func writeTimings(a *App, document report.Document) error {
	for _, c := range document.Collectors {
		status := "ok"
		if c.Error != "" {
			status = c.Error
		}
		if _, err := fmt.Fprintf(a.Stderr, "collector %-10s %5dms  %s\n", report.VisibleText(c.Name), c.DurationMs, report.VisibleText(status)); err != nil {
			return fmt.Errorf("write collector timings: %w", err)
		}
	}
	return nil
}

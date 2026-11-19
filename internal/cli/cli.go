// Package cli wires flags, collectors and reports into the envdoctor binary.
//
// The package is deliberately testable: streams, the clock, the collectors and
// the rule engine are all injected, so the CLI tests do not depend on the
// machine they run on.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/shellghostt/envdoctor/internal/bundle"
	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/shellghostt/envdoctor/internal/report"
)

// Process exit codes. PLAN section 8 is the contract.
const (
	ExitOK          = 0
	ExitWarning     = 1
	ExitError       = 2
	ExitToolFailure = 3
	ExitUsage       = 4
)

// App is one invocation of the binary.
type App struct {
	Version     string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	Getenv      func(string) string
	LookupEnv   func(string) (string, bool)
	Context     context.Context
	Terminals   func() report.TerminalState
	Interactive func(context.Context, io.Reader, io.Writer, report.Document) error
	Now         func() time.Time
	Collectors  []collector.Collector
	HomeDir     func() (string, error)
	Engine      *diagnostic.Engine

	exitCode   int
	bundleMode bool
}

// toolFailure marks an error as a failure of the tool itself rather than of the
// user's machine. It maps to exit code 3.
type toolFailure struct{ err error }

func (e toolFailure) Error() string { return e.err.Error() }
func (e toolFailure) Unwrap() error { return e.err }

// Main runs the CLI and returns the process exit code. It never calls os.Exit,
// which keeps it usable from tests.
func Main(args []string, app App) int {
	app.applyDefaults()
	for _, arg := range args[1:] {
		if arg == "bundle" || arg == "--from-bundle" || strings.HasPrefix(arg, "--from-bundle=") {
			app.bundleMode = true
		}
	}

	root := app.newRoot()
	root.SetArgs(args[1:])

	if err := root.ExecuteContext(app.Context); err != nil {
		var failure toolFailure
		failed := errors.As(err, &failure)
		message := err.Error()
		if app.bundleMode {
			message = "invalid bundle command options"
			if failed {
				message = "diagnostic bundle operation failed"
				for _, reason := range []error{bundle.ErrInvalid, bundle.ErrBounds, bundle.ErrVersion, bundle.ErrIntegrity, bundle.ErrEvidence, bundle.ErrRead, bundle.ErrWrite, context.Canceled, context.DeadlineExceeded} {
					if errors.Is(err, reason) {
						message = reason.Error()
						break
					}
				}
			}
		} else {
			home, _ := app.HomeDir()
			message = collector.RedactText(message, home)
		}
		_, _ = fmt.Fprintf(app.Stderr, "error: %s\n", report.VisibleText(message))
		if failed {
			return ExitToolFailure
		}
		_, _ = fmt.Fprintln(app.Stderr, "Run 'envdoctor --help' for usage.")
		return ExitUsage
	}
	return app.exitCode
}

func (a *App) applyDefaults() {
	if a.Context == nil {
		a.Context = context.Background()
	}
	if a.Stdout == nil {
		a.Stdout = io.Discard
	}
	if a.Stderr == nil {
		a.Stderr = io.Discard
	}
	if a.Getenv == nil {
		a.Getenv = os.Getenv
	}
	if a.LookupEnv == nil {
		a.LookupEnv = os.LookupEnv
	}
	if a.Interactive == nil {
		a.Interactive = report.WriteInteractive
	}
	if a.Now == nil {
		a.Now = time.Now
	}
	if a.HomeDir == nil {
		a.HomeDir = os.UserHomeDir
	}
	if a.Collectors == nil {
		a.Collectors = DefaultCollectors()
	}
	if a.Engine == nil {
		a.Engine = diagnostic.DefaultEngine()
	}
}

// DefaultCollectors preserves the M2 collectors and adds project/proxy evidence.
// Proxy collection performs no network work without an explicit diagnose flag.
func DefaultCollectors() []collector.Collector {
	return []collector.Collector{
		collector.NewPlatformCollector(),
		collector.NewShellCollector(),
		collector.NewPathCollector(),
		collector.NewRuntimeCollector(),
		collector.PortCollector{},
		collector.GitCollector{},
		collector.DockerCollector{},
		collector.DependencyCollector{},
		collector.ProxyCollector{},
	}
}

func (a *App) newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "envdoctor",
		Short: "Diagnose development environment problems",
		Long: "EnvDoctor inspects this machine, applies explicit rules and explains what is wrong.\n" +
			"It never modifies anything unless you run a fix command and confirm it.",
		Version:       a.Version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetOut(a.Stdout)
	root.SetErr(a.Stderr)
	root.SetIn(a.Stdin)

	root.AddCommand(a.newDiagnoseCommand())
	root.AddCommand(a.newBundleCommand())
	root.AddCommand(a.newVersionCommand())
	root.AddCommand(a.newExplainCommand())
	root.AddCommand(a.newRulesCommand())
	return root
}

func terminalStream(stream any) bool {
	file, ok := stream.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(int(file.Fd()))
}

func (a *App) terminalState() report.TerminalState {
	if a.Terminals != nil {
		return a.Terminals()
	}
	_, noColor := a.LookupEnv("NO_COLOR")
	return report.TerminalState{Input: terminalStream(a.Stdin), Output: terminalStream(a.Stdout), Error: terminalStream(a.Stderr), Term: a.Getenv("TERM"), NoColor: noColor}
}

func (a *App) newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(a.Stdout, "%s %s\n", "envdoctor", a.Version)
			if err != nil {
				return toolFailure{err}
			}
			return nil
		},
	}
}

// Package cli wires flags, collectors and reports into the envdoctor binary.
//
// The package is deliberately testable: streams, the clock, the collectors and
// the rule engine are all injected, so the CLI tests do not depend on the
// machine they run on.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
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
	Version    string
	Stdout     io.Writer
	Stderr     io.Writer
	Getenv     func(string) string
	Now        func() time.Time
	Collectors []collector.Collector
	HomeDir    func() (string, error)
	Engine     *diagnostic.Engine

	exitCode int
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

	root := app.newRoot()
	root.SetArgs(args[1:])

	if err := root.Execute(); err != nil {
		fmt.Fprintf(app.Stderr, "error: %v\n", err)
		var failure toolFailure
		if errors.As(err, &failure) {
			return ExitToolFailure
		}
		fmt.Fprintln(app.Stderr, "Run 'envdoctor --help' for usage.")
		return ExitUsage
	}
	return app.exitCode
}

func (a *App) applyDefaults() {
	if a.Stdout == nil {
		a.Stdout = io.Discard
	}
	if a.Stderr == nil {
		a.Stderr = io.Discard
	}
	if a.Getenv == nil {
		a.Getenv = os.Getenv
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
		a.Engine = diagnostic.NewEngine()
	}
}

// DefaultCollectors returns the collectors implemented in this build. M1 has
// three; runtimes, ports, Git and Docker arrive in M2 and M3.
func DefaultCollectors() []collector.Collector {
	return []collector.Collector{
		collector.NewPlatformCollector(),
		collector.NewShellCollector(),
		collector.NewPathCollector(),
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

	root.AddCommand(a.newDiagnoseCommand())
	root.AddCommand(a.newVersionCommand())
	return root
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

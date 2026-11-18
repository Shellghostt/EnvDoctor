package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/shellghostt/envdoctor/internal/bundle"
	"github.com/shellghostt/envdoctor/internal/config"
)

func (a *App) newBundleCommand() *cobra.Command {
	opts := config.Options{}
	output := "envdoctor-bundle.zip"
	includeLogs := false
	cmd := &cobra.Command{
		Use: "bundle", Short: "Publish a redacted sharing bundle of the complete builtin diagnosis",
		Long: "Capture approved diagnostic facts and publish a bounded ZIP atomically.\n" +
			"Git identity becomes presence facts; paths and hosts become capture-local aliases.\n" +
			"--include-logs includes only structured collector statuses/timings, never raw logs.\n" +
			"Publication success exits 0; the capture's diagnosis exit is reference metadata.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.Validate(); err != nil {
				return err
			}
			if output == "" {
				return fmt.Errorf("--output requires a file")
			}
			doc, err := a.captureDocument(cmd.Context(), opts, false)
			if err != nil {
				return toolFailure{err}
			}
			data, err := bundle.Build(cmd.Context(), doc, bundle.Options{IncludeLogs: includeLogs})
			if err != nil {
				return toolFailure{err}
			}
			// Report bounded progress before committing the destination. A broken
			// status stream must not turn a successful rename into a failed command
			// that has already replaced the user's previous capture.
			status := fmt.Sprintf("Writing diagnostic bundle (%d bytes); original diagnosis exit %d.\n", len(data), doc.ExitCode)
			n, err := io.WriteString(a.Stdout, status)
			if err == nil && n != len(status) {
				err = io.ErrShortWrite
			}
			if err != nil {
				return toolFailure{err}
			}
			if err := bundle.Publish(cmd.Context(), output, data); err != nil {
				return toolFailure{err}
			}
			a.exitCode = ExitOK
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "destination ZIP file")
	cmd.Flags().BoolVar(&includeLogs, "include-logs", false, "include only allowlisted structured collector statuses/timings")
	bindCaptureFlags(cmd, &opts)
	return cmd
}

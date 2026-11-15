package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/diagnostic"
)

// WriteText renders the human report. There is no colour and no cursor
// movement: the output is stable and safe to paste into an issue.
func WriteText(w io.Writer, document Document) error {
	var b strings.Builder

	fmt.Fprintf(&b, "EnvDoctor %s - environment diagnosis\n", document.Tool.Version)
	fmt.Fprintf(&b, "Generated %s\n\n", document.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))

	writePlatform(&b, document.Snapshot)
	writePath(&b, document.Snapshot.Path)
	writeTools(&b, document.Snapshot)
	fmt.Fprintf(&b, "Rules      %d registered in this build\n\n", document.Rules)
	writeDegraded(&b, document)
	writeFindings(&b, document.Findings)

	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("write text report: %w", err)
	}
	return nil
}

func writeTools(b *strings.Builder, snapshot collector.Snapshot) {
	for _, runtime := range snapshot.Runtimes {
		version := runtime.Version
		if version == "" {
			version = "version unknown"
		}
		status := "candidate"
		if runtime.Active {
			status = "active"
		}
		if runtime.Expected {
			status += ", selected"
		}
		if runtime.Missing {
			status += ", missing target"
		}
		if runtime.Resolution == "unknown" {
			status += ", resolution unknown"
		}
		fmt.Fprintf(b, "Runtime    %s %s (%s; %s) %s\n", runtime.Name, version, runtime.Manager, status, runtime.Path)
	}
	if snapshot.Git != nil {
		autocrlf := snapshot.Git.Autocrlf
		if autocrlf == "" {
			autocrlf = "unset"
		}
		fmt.Fprintf(b, "Git        %s; core.autocrlf=%s\n", snapshot.Git.Version, autocrlf)
	}
	if snapshot.Docker != nil {
		if !snapshot.Docker.Installed {
			b.WriteString("Docker     CLI not found in known installation locations\n")
		} else {
			state := "unreachable"
			if snapshot.Docker.DaemonReachable {
				state = "reachable"
			}
			if snapshot.Docker.ProbeStatus != "" {
				state = snapshot.Docker.ProbeStatus
			}
			fmt.Fprintf(b, "Docker     %s; %s engine %s\n", snapshot.Docker.CLIVersion, snapshot.Docker.Context, state)
			if snapshot.Docker.SelectedContext != "" {
				fmt.Fprintf(b, "Context    %s; %s (metadata only)\n", snapshot.Docker.SelectedContext, snapshot.Docker.SelectedHost)
			}
		}
	}
	if snapshot.Dependencies != nil {
		for _, dependency := range snapshot.Dependencies.Requirements {
			fmt.Fprintf(b, "Dependency %s (%s): %s %s\n", dependency.Name, dependency.Kind, dependency.Status, dependency.Path)
		}
	}
	for _, proxy := range snapshot.Proxies {
		fmt.Fprintf(b, "Proxy      %s %s; reachability %s\n", proxy.Variable, proxy.URL, proxy.Probe)
	}
	if len(snapshot.Runtimes) > 0 || snapshot.Git != nil || snapshot.Docker != nil {
		b.WriteString("\n")
	}
}

func writePlatform(b *strings.Builder, snapshot collector.Snapshot) {
	if snapshot.Platform == nil {
		b.WriteString("Platform   unknown (collector did not run)\n")
	} else {
		platform := snapshot.Platform
		version := platform.Version
		if version == "" {
			version = "version unknown"
		}
		fmt.Fprintf(b, "Platform   %s %s %s\n", platform.OS, version, platform.Arch)
		fmt.Fprintf(b, "Elevated   %s\n", yesNo(platform.Elevated))
	}

	if snapshot.Shell == nil {
		b.WriteString("Shell      unknown (collector did not run)\n")
	} else if snapshot.Shell.Path == "" {
		fmt.Fprintf(b, "Shell      %s\n", snapshot.Shell.Name)
	} else {
		fmt.Fprintf(b, "Shell      %s (%s)\n", snapshot.Shell.Name, snapshot.Shell.Path)
	}
}

func writePath(b *strings.Builder, path *collector.PathInfo) {
	if path == nil {
		b.WriteString("PATH       unknown (collector did not run)\n\n")
		return
	}

	missing, duplicates := 0, 0
	var missingEntries []string
	for _, entry := range path.Entries {
		if !entry.Exists {
			missing++
			missingEntries = append(missingEntries, entry.Raw)
		}
		if entry.Duplicate {
			duplicates++
		}
	}

	fmt.Fprintf(b, "PATH       %d entries, %d missing, %d duplicate\n\n", len(path.Entries), missing, duplicates)

	if len(missingEntries) > 0 {
		b.WriteString("Missing PATH entries:\n")
		for _, entry := range missingEntries {
			fmt.Fprintf(b, "  - %s\n", entry)
		}
		b.WriteString("\n")
	}
}

func writeDegraded(b *strings.Builder, document Document) {
	var failed []Collector
	for _, c := range document.Collectors {
		if c.Error != "" {
			failed = append(failed, c)
		}
	}
	if len(failed) == 0 {
		return
	}

	b.WriteString("Degraded collectors (their rules were skipped):\n")
	for _, c := range failed {
		fmt.Fprintf(b, "  - %s: %s\n", c.Name, c.Error)
	}
	b.WriteString("\n")
}

func writeFindings(b *strings.Builder, findings []diagnostic.Finding) {
	if len(findings) == 0 {
		b.WriteString("No findings.\n")
		return
	}

	for _, finding := range findings {
		fmt.Fprintf(b, "[%s] %s\n\n", strings.ToUpper(string(finding.Severity)), finding.Title)

		b.WriteString("Detected:\n")
		for _, item := range finding.Detected {
			fmt.Fprintf(b, "  - %s\n", item)
		}

		if finding.Cause != "" {
			fmt.Fprintf(b, "\nPossible cause:\n  %s\n", finding.Cause)
		}

		if finding.Fix != nil {
			fmt.Fprintf(b, "\nSuggested fix:\n  %s\n", finding.Fix.Summary)
			if len(finding.Fix.Commands) > 0 {
				b.WriteString("\nRun:\n")
				for _, command := range finding.Fix.Commands {
					fmt.Fprintf(b, "  %s\n", command)
				}
			}
		}

		if finding.Verify != "" {
			fmt.Fprintf(b, "\nVerify:\n  %s\n", finding.Verify)
		}

		fmt.Fprintf(b, "\nRule: %s\n\n", finding.RuleID)
	}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

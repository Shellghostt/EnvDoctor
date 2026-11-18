package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strconv"
	"strings"

	"github.com/shellghostt/envdoctor/internal/diagnostic"
	"github.com/shellghostt/envdoctor/internal/report"
)

var omissionFields = []string{"platform", "platform.version", "shell.name", "shell.path", "path.locations", "runtimes.paths", "runtimes.version", "runtimes.name", "runtimes.manager", "runtimes.resolution", "runtimes.environment", "network.ownership", "docker.context", "docker.endpoint", "docker.version", "docker.api", "git.identity", "git.origins", "git.version", "git.autocrlf", "dependencies.locations", "dependencies.requirement", "dependencies.packageManager", "dependencies.lockfile", "dependencies.gitAutocrlf", "pythonEnvironment.locations", "proxy.endpoint", "proxy.url", "collector.errors", "tool.version"}

func validToken(value string, kind string, optional bool) bool {
	if value == "" {
		return optional
	}
	var digits string
	switch kind {
	case "path":
		if !pathToken.MatchString(value) {
			return false
		}
		digits = value[5:]
	case "env":
		if !environmentToken.MatchString(value) {
			return false
		}
		digits = value[4:]
	case "host":
		if !hostToken.MatchString(value) {
			return false
		}
		digits = strings.TrimSuffix(value[5:], ".invalid")
	default:
		return false
	}
	n, err := strconv.Atoi(digits)
	return err == nil && n >= 1 && n <= 4096
}

func validateSnapshot(s SharingSnapshot) error {
	if s.SchemaVersion != 1 {
		return ErrVersion
	}
	if s.CollectedAt.IsZero() {
		return ErrInvalid
	}
	if len(s.Runtimes) > 128 || len(s.Ports) > 1024 || len(s.RequestedPorts) > 1024 || len(s.Proxies) > 3 || len(s.Errors) > 32 || s.Path != nil && len(s.Path.Entries) > 1024 || s.Dependencies != nil && len(s.Dependencies.Requirements) > 32 {
		return ErrBounds
	}
	if p := s.Platform; p != nil {
		if !allowed(p.OS, "windows", "linux", "darwin") || !allowed(p.Arch, "amd64", "arm64", "386", "arm", "ppc64le", "s390x", "riscv64") || !version(p.Version) {
			return ErrInvalid
		}
	}
	if s.Shell != nil && !knownShell(s.Shell.Name) {
		return ErrInvalid
	}
	if p := s.Path; p != nil {
		for i, e := range p.Entries {
			if !validToken(e.Token, "path", false) || e.Position != i || e.IsDir && !e.Exists {
				return ErrInvalid
			}
		}
	}
	for _, r := range s.Runtimes {
		if !allowed(r.Name, "python", "node") || !knownManager(r.Manager) || !version(r.Version) || !validToken(r.PathToken, "path", false) || !validToken(r.ResolvedToken, "path", true) || !validToken(r.EnvironmentToken, "env", true) || !allowed(r.Resolution, "", "known", "unknown") {
			return ErrInvalid
		}
	}
	for _, p := range s.RequestedPorts {
		if p < 1 || p > 65535 {
			return ErrInvalid
		}
	}
	for _, p := range s.Ports {
		if p.Port < 1 || p.Port > 65535 || !allowed(p.Protocol, "tcp", "udp") || !allowed(p.State, "listen", "established", "closed", "unknown") {
			return ErrInvalid
		}
	}
	if d := s.Docker; d != nil {
		if !version(d.CLIVersion) || !allowed(d.ProbeStatus, "", "absent", "available", "unavailable", "unknown", "denied", "unsupported") {
			return ErrInvalid
		}
		for _, v := range []string{d.ClientAPI, d.ClientMinAPI, d.ClientDefaultAPI, d.ServerAPI, d.ServerMinAPI, d.APIOverride} {
			if !knownAPI(v) {
				return ErrInvalid
			}
		}
		if d.ProbeStatus == "available" && (!d.Installed || !d.DaemonReachable) || d.ProbeStatus == "absent" && d.Installed {
			return ErrInvalid
		}
	}
	if g := s.Git; g != nil {
		if !version(g.Version) || !knownAutocrlf(g.Autocrlf) {
			return ErrInvalid
		}
	}
	if d := s.Dependencies; d != nil {
		if !validToken(d.ProjectToken, "path", true) || !knownAutocrlf(d.GitAutocrlf) {
			return ErrInvalid
		}
		for _, r := range d.Requirements {
			if !knownDependency(r.Kind, r.Name) || !allowed(r.Status, "present", "missing", "unknown") || !validToken(r.PathToken, "path", true) {
				return ErrInvalid
			}
		}
		if p := d.Package; p != nil {
			for _, m := range []*PackageManager{p.Declared, p.Runner} {
				if m != nil && (!knownPackageManager(m.Name) || m.Version != "" && !exactVersion(m.Version)) {
					return ErrInvalid
				}
			}
			if p.Declared != nil && !exactVersion(p.Declared.Version) {
				return ErrInvalid
			}
			for _, lock := range p.Lockfiles {
				if !knownLockfile(lock) {
					return ErrInvalid
				}
			}
		}
	}
	if e := s.PythonEnvironment; e != nil {
		if !validToken(e.RootToken, "env", true) || !validToken(e.CanonicalToken, "env", true) || e.MetadataKnown && e.CanonicalToken == "" {
			return ErrInvalid
		}
	}
	seen := map[string]bool{}
	for _, p := range s.Proxies {
		if seen[p.Variable] || !allowed(p.Variable, "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY") || !allowed(p.Scheme, "http", "https", "socks5", "socks5h") || !validToken(p.HostToken, "host", false) || p.Port < 1 || p.Port > 65535 || !allowed(p.Probe, "untested", "reachable", "unreachable", "unknown", "invalid") {
			return ErrInvalid
		}
		seen[p.Variable] = true
	}
	seen = map[string]bool{}
	for _, f := range s.Errors {
		if !knownCollector(f.Collector) || f.Reason != "unavailable" || seen[f.Collector] {
			return ErrInvalid
		}
		seen[f.Collector] = true
	}
	return nil
}

func validateManifest(m Manifest, all map[string][]byte) error {
	if m.BundleVersion != 1 || m.SnapshotVersion != 1 || m.ReportVersion != 1 || m.RedactionPolicy != Policy {
		return ErrVersion
	}
	if m.Tool.Name != "envdoctor" || m.Tool.Version == "" || m.Tool.Version != "dev" && !numericVersion.MatchString(m.Tool.Version) || m.CapturedAt.IsZero() || m.CaptureExitCode < 0 || m.CaptureExitCode > 3 {
		return ErrInvalid
	}
	if len(m.Omissions) > 256 || len(m.Members) > MaxEntries-1 {
		return ErrBounds
	}
	seenOmissions := map[report.Omission]bool{}
	for _, o := range m.Omissions {
		if !allowed(o.Field, omissionFields...) || !allowed(o.Reason, "identifying_values", "raw_error", "unsupported_value", "unavailable_evidence") || seenOmissions[o] {
			return ErrInvalid
		}
		seenOmissions[o] = true
	}
	if len(m.Members) != len(all)-1 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, member := range m.Members {
		body, ok := all[member.Name]
		if !ok || member.Name == "manifest.json" || seen[member.Name] || member.Size < 0 || member.Size != len(body) {
			return ErrIntegrity
		}
		seen[member.Name] = true
		digest := sha256.Sum256(body)
		if member.SHA256 != hex.EncodeToString(digest[:]) {
			return ErrIntegrity
		}
	}
	return nil
}

func makeVersions(s SharingSnapshot, tool report.Tool) Versions {
	v := Versions{SchemaVersion: 1, Tool: tool, Runtimes: []RuntimeVersion{}}
	seen := map[RuntimeVersion]bool{}
	for _, r := range s.Runtimes {
		item := RuntimeVersion{Name: r.Name, Version: r.Version}
		if r.Version != "" && !seen[item] {
			v.Runtimes = append(v.Runtimes, item)
			seen[item] = true
		}
	}
	if s.Git != nil {
		v.Git = s.Git.Version
	}
	if s.Docker != nil {
		v.Docker = s.Docker.CLIVersion
	}
	return v
}

func sourceFor(m Manifest) report.Source {
	return report.Source{Kind: "bundle", CaptureTool: m.Tool, CapturedAt: m.CapturedAt, RedactionPolicy: m.RedactionPolicy, CaptureExitCode: m.CaptureExitCode, Omissions: m.Omissions}
}

func validateReference(doc report.Document, s SharingSnapshot, m Manifest, v Versions, diags *Diagnostics) error {
	if doc.SchemaVersion != 1 || v.SchemaVersion != 1 {
		return ErrVersion
	}
	expectedSource := sourceFor(m)
	if doc.Source == nil || !reflect.DeepEqual(*doc.Source, expectedSource) || doc.Tool != m.Tool || !doc.GeneratedAt.Equal(m.CapturedAt) || !s.CollectedAt.Equal(m.CapturedAt) || !reflect.DeepEqual(doc.Snapshot, engineSnapshot(s)) || !reflect.DeepEqual(v, makeVersions(s, m.Tool)) {
		return ErrInvalid
	}
	if doc.ExitCode < 0 || doc.ExitCode > 3 || doc.Rules < 0 || doc.Rules > 4096 {
		return ErrInvalid
	}
	summary := report.Summary{Total: len(doc.Findings)}
	for _, f := range doc.Findings {
		if err := f.Validate(); err != nil {
			return ErrInvalid
		}
		switch f.Severity {
		case diagnostic.SeverityInfo:
			summary.Info++
		case diagnostic.SeverityWarning:
			summary.Warning++
		case diagnostic.SeverityError:
			summary.Error++
		case diagnostic.SeverityCritical:
			summary.Critical++
		}
	}
	if summary != doc.Summary {
		return ErrInvalid
	}
	failed := map[string]bool{}
	for _, f := range s.Errors {
		failed[f.Collector] = true
	}
	seen := map[string]bool{}
	for _, c := range doc.Collectors {
		if !knownCollector(c.Name) || seen[c.Name] || c.DurationMs < 0 || c.DurationMs > maxDurationMs || c.Error != "" && c.Error != unavailableNotice || (c.Error != "") != failed[c.Name] {
			return ErrInvalid
		}
		seen[c.Name] = true
	}
	for name := range failed {
		if !seen[name] {
			return ErrInvalid
		}
	}
	if diags != nil {
		if diags.SchemaVersion != 1 {
			return ErrVersion
		}
		if len(diags.Collectors) > 32 {
			return ErrBounds
		}
		seen = map[string]bool{}
		for _, c := range diags.Collectors {
			if !knownCollector(c.Collector) || seen[c.Collector] || c.DurationMs < 0 || c.DurationMs > maxDurationMs || !allowed(c.Status, "available", "unavailable") || (c.Status == "unavailable") != failed[c.Collector] {
				return ErrInvalid
			}
			seen[c.Collector] = true
		}
		for name := range failed {
			if !seen[name] {
				return ErrInvalid
			}
		}
	}
	return nil
}

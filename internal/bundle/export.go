package bundle

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/report"
)

var numericVersion = regexp.MustCompile(`^[0-9]{1,9}(?:\.[0-9]{1,9}){0,3}$`)
var windowsVersion = regexp.MustCompile(`^([0-9]{1,9}) \(build ([0-9]{1,9})\)$`)
var windowsGitVersion = regexp.MustCompile(`^([0-9]{1,9}\.[0-9]{1,9}\.[0-9]{1,9})\.windows\.[0-9]{1,9}$`)
var pathToken = regexp.MustCompile(`^path-[0-9]{4}$`)
var environmentToken = regexp.MustCompile(`^env-[0-9]{4}$`)
var hostToken = regexp.MustCompile(`^host-[0-9]{4}\.invalid$`)

var collectorNames = []string{"platform", "shell", "path", "runtimes", "network", "docker", "git", "deps", "proxy"}

const unavailableNotice = "captured collector unavailable"
const maxDurationMs = 60_000

type aliases struct {
	prefix string
	values map[string]string
}

func (a *aliases) token(value string) string {
	if token, ok := a.values[value]; ok {
		return token
	}
	token := fmt.Sprintf("%s-%04d", a.prefix, len(a.values)+1)
	a.values[value] = token
	return token
}

func (a *aliases) optional(value string) string {
	if value == "" {
		return ""
	}
	return a.token(value)
}

type exporter struct {
	snapshot                   SharingSnapshot
	omissions                  []report.Omission
	seenOmissions              map[report.Omission]bool
	failed                     map[string]bool
	paths, environments, hosts aliases
	windows                    bool
}

func (e *exporter) omit(field, reason string) {
	omission := report.Omission{Field: field, Reason: reason}
	if !e.seenOmissions[omission] {
		e.omissions = append(e.omissions, omission)
		e.seenOmissions[omission] = true
	}
}

func (e *exporter) unavailable(name, field string) {
	e.failed[name] = true
	e.omit(field, "unsupported_value")
}

func (e *exporter) environment(value string) string {
	if value == "" {
		return ""
	}
	// This is the same pure normalization used by ENV011, with the CAPTURED
	// platform. filepath, os and the receiving machine never participate.
	value = path.Clean(strings.ReplaceAll(value, `\`, "/"))
	if e.windows {
		value = strings.ToLower(value)
	}
	return e.environments.token(value)
}

func allowed(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}
func knownCollector(value string) bool { return allowed(value, collectorNames...) }
func version(value string) bool        { return value == "" || numericVersion.MatchString(value) }
func exactVersion(value string) bool {
	if !numericVersion.MatchString(value) || len(strings.Split(value, ".")) != 3 {
		return false
	}
	for _, part := range strings.Split(value, ".") {
		if len(part) > 1 && part[0] == '0' {
			return false
		}
	}
	return true
}
func toolVersion(value string) string {
	value = strings.TrimPrefix(value, "v")
	if numericVersion.MatchString(value) {
		return value
	}
	return "dev"
}
func knownShell(value string) bool {
	return allowed(value, "powershell", "pwsh", "cmd", "bash", "zsh", "fish", "sh", "dash", "ksh", "tcsh", "csh", "nu")
}
func knownManager(value string) bool {
	return allowed(value, "system", "venv", "pyenv", "nvm", "fnm", "volta")
}
func knownPackageManager(value string) bool { return allowed(value, "npm", "pnpm", "yarn", "bun") }
func knownDependency(kind, name string) bool {
	return kind == "tool" && allowed(name, "cmake", "make", "gcc", "pkg-config") || kind == "library" && name == "openssl"
}
func knownLockfile(value string) bool {
	return allowed(value, "package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb")
}
func knownAutocrlf(value string) bool { return allowed(value, "", "true", "false", "input") }
func knownAPI(value string) bool {
	if value == "" {
		return true
	}
	major, minor, ok := strings.Cut(value, ".")
	if !ok || major != "1" || len(minor) == 0 || len(minor) > 6 {
		return false
	}
	for _, ch := range minor {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func sourceBounds(ctx context.Context, d report.Document) error {
	s := d.Snapshot
	if len(s.Runtimes) > 128 || len(s.Ports) > 1024 || len(s.RequestedPorts) > 1024 || len(s.Proxies) > 3 || len(s.Errors) > 32 || len(d.Collectors) > 32 || s.Path != nil && len(s.Path.Entries) > 1024 || s.Dependencies != nil && len(s.Dependencies.Requirements) > 32 {
		return ErrBounds
	}
	if s.Dependencies != nil && s.Dependencies.Package != nil && len(s.Dependencies.Package.Lockfiles) > 4096 {
		return ErrBounds
	}
	// Check lengths without making copies or searching raw strings for secrets.
	// Only source facts/timings/tool metadata matter; original finding text is
	// never traversed or exported.
	var visit func(reflect.Value) error
	visit = func(v reflect.Value) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				return visit(v.Elem())
			}
		case reflect.String:
			if len(v.String()) > 4096 || !utf8.ValidString(v.String()) {
				return ErrBounds
			}
		case reflect.Struct:
			if v.Type() == reflect.TypeFor[time.Time]() {
				return nil
			}
			for i := 0; i < v.NumField(); i++ {
				if err := visit(v.Field(i)); err != nil {
					return err
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				if err := visit(v.Index(i)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, value := range []any{s, d.Collectors, d.Tool} {
		if err := visit(reflect.ValueOf(value)); err != nil {
			return err
		}
	}
	return nil
}

func export(ctx context.Context, d report.Document) (SharingSnapshot, report.Source, []collector.Timing, Diagnostics, error) {
	if !currentSourceModel() {
		return SharingSnapshot{}, report.Source{}, nil, Diagnostics{}, ErrEvidence
	}
	if err := sourceBounds(ctx, d); err != nil {
		return SharingSnapshot{}, report.Source{}, nil, Diagnostics{}, err
	}
	s := d.Snapshot
	if s.SchemaVersion != 1 || d.SchemaVersion != 1 {
		return SharingSnapshot{}, report.Source{}, nil, Diagnostics{}, ErrVersion
	}
	if s.CollectedAt.IsZero() || s.CollectedAt.Year() < 1 || s.CollectedAt.Year() > 9999 || d.Tool.Name != "envdoctor" || d.ExitCode < 0 || d.ExitCode > 3 {
		return SharingSnapshot{}, report.Source{}, nil, Diagnostics{}, ErrEvidence
	}
	e := &exporter{snapshot: SharingSnapshot{SchemaVersion: 1, CollectedAt: s.CollectedAt.UTC(), NoProxyConfigured: s.NoProxyConfigured},
		omissions: []report.Omission{}, seenOmissions: map[report.Omission]bool{}, failed: map[string]bool{},
		paths: aliases{"path", map[string]string{}}, environments: aliases{"env", map[string]string{}}, hosts: aliases{"host", map[string]string{}},
		windows: s.Platform != nil && s.Platform.OS == "windows"}
	for _, failure := range s.Errors {
		if !knownCollector(failure.Collector) {
			return SharingSnapshot{}, report.Source{}, nil, Diagnostics{}, ErrEvidence
		}
		e.failed[failure.Collector] = true
		e.omit("collector.errors", "raw_error")
	}
	if p := s.Platform; p != nil {
		if !allowed(p.OS, "windows", "linux", "darwin") || !allowed(p.Arch, "amd64", "arm64", "386", "arm", "ppc64le", "s390x", "riscv64") {
			e.unavailable("platform", "platform")
			e.unavailable("runtimes", "runtimes.environment")
			if s.Dependencies != nil {
				e.unavailable("deps", "dependencies.requirement")
			}
		} else {
			v := p.Version
			if parts := windowsVersion.FindStringSubmatch(v); parts != nil && p.OS == "windows" {
				v = parts[1] + "." + parts[2]
			}
			if !version(v) {
				v = ""
				e.omit("platform.version", "unsupported_value")
			}
			e.snapshot.Platform = &Platform{OS: p.OS, Arch: p.Arch, Version: v, Elevated: p.Elevated}
		}
	}
	if sh := s.Shell; sh != nil {
		if knownShell(sh.Name) {
			e.snapshot.Shell = &Shell{Name: sh.Name}
		} else {
			e.unavailable("shell", "shell.name")
		}
		if sh.Path != "" {
			e.omit("shell.path", "identifying_values")
		}
	}
	if p := s.Path; p != nil {
		e.snapshot.Path = &PathInfo{Entries: make([]PathEntry, 0, len(p.Entries))}
		for _, entry := range p.Entries {
			if entry.Position < 0 || entry.Position > 1023 {
				return SharingSnapshot{}, report.Source{}, nil, Diagnostics{}, ErrEvidence
			}
			e.snapshot.Path.Entries = append(e.snapshot.Path.Entries, PathEntry{Token: e.paths.token(entry.Raw), Exists: entry.Exists, IsDir: entry.IsDir, Position: entry.Position, Duplicate: entry.Duplicate})
		}
		if len(p.Entries) > 0 {
			e.omit("path.locations", "identifying_values")
		}
	}
	for _, r := range s.Runtimes {
		if !allowed(r.Name, "python", "node") || r.Path == "" {
			e.unavailable("runtimes", "runtimes.name")
			continue
		}
		v, manager, resolution := r.Version, r.Manager, r.Resolution
		if !version(v) {
			v = ""
			e.unavailable("runtimes", "runtimes.version")
		}
		if !knownManager(manager) {
			e.unavailable("runtimes", "runtimes.manager")
			continue
		}
		if !allowed(resolution, "", "known", "unknown") {
			resolution = "unknown"
			e.unavailable("runtimes", "runtimes.resolution")
		}
		e.snapshot.Runtimes = append(e.snapshot.Runtimes, Runtime{Name: r.Name, Version: v, Manager: manager, Active: r.Active, Expected: r.Expected, Missing: r.Missing,
			PathToken: e.paths.token(r.Path), ResolvedToken: e.paths.optional(r.ResolvedPath), Resolution: resolution, EnvironmentToken: e.environment(r.EnvironmentRoot)})
		e.omit("runtimes.paths", "identifying_values")
	}
	e.snapshot.RequestedPorts = append([]int(nil), s.RequestedPorts...)
	for _, p := range s.RequestedPorts {
		if p < 1 || p > 65535 {
			return SharingSnapshot{}, report.Source{}, nil, Diagnostics{}, ErrEvidence
		}
	}
	for _, p := range s.Ports {
		if p.Port < 1 || p.Port > 65535 || !allowed(p.Protocol, "tcp", "udp") || !allowed(p.State, "listen", "established", "closed", "unknown") {
			e.unavailable("network", "network.ownership")
			continue
		}
		e.snapshot.Ports = append(e.snapshot.Ports, Port{Port: p.Port, Protocol: p.Protocol, State: p.State})
		if p.PID != 0 || p.Process != "" {
			e.omit("network.ownership", "identifying_values")
		}
	}
	if d := s.Docker; d != nil {
		v := d.CLIVersion
		if !version(v) {
			v = ""
			e.omit("docker.version", "unsupported_value")
		}
		probe := d.ProbeStatus
		if !allowed(probe, "", "absent", "available", "unavailable", "unknown", "denied", "unsupported") {
			probe = "unknown"
			e.unavailable("docker", "docker.api")
		}
		out := &Docker{CLIVersion: v, Installed: d.Installed, DaemonReachable: d.DaemonReachable, ProbeStatus: probe}
		for _, api := range []struct {
			raw  string
			dest *string
		}{{d.ClientAPI, &out.ClientAPI}, {d.ClientMinAPI, &out.ClientMinAPI}, {d.ClientDefaultAPI, &out.ClientDefaultAPI}, {d.ServerAPI, &out.ServerAPI}, {d.ServerMinAPI, &out.ServerMinAPI}, {d.APIOverride, &out.APIOverride}} {
			if knownAPI(api.raw) {
				*api.dest = api.raw
			} else {
				e.unavailable("docker", "docker.api")
			}
		}
		e.snapshot.Docker = out
		if d.Context != "" || d.SelectedContext != "" {
			e.omit("docker.context", "identifying_values")
		}
		if d.SelectedHost != "" {
			e.omit("docker.endpoint", "identifying_values")
		}
	}
	if g := s.Git; g != nil {
		v, crlf := g.Version, g.Autocrlf
		if parts := windowsGitVersion.FindStringSubmatch(v); parts != nil {
			v = parts[1]
		}
		if !version(v) {
			v = ""
			e.unavailable("git", "git.version")
		}
		if !knownAutocrlf(crlf) {
			crlf = ""
			e.unavailable("git", "git.autocrlf")
		}
		e.snapshot.Git = &Git{Version: v, IdentityChecked: g.IdentityChecked, UserNamePresent: g.UserName != nil && strings.TrimSpace(*g.UserName) != "", UserEmailPresent: g.UserEmail != nil && strings.TrimSpace(*g.UserEmail) != "", Autocrlf: crlf, AutocrlfOriginKnown: g.AutocrlfOrigin != ""}
		if g.UserName != nil || g.UserEmail != nil {
			e.omit("git.identity", "identifying_values")
		}
		if g.UserNameOrigin != "" || g.UserEmailOrigin != "" || g.AutocrlfOrigin != "" {
			e.omit("git.origins", "identifying_values")
		}
	}
	if d := s.Dependencies; d != nil {
		out := &Dependencies{ProjectToken: e.paths.optional(d.ProjectRoot), RequirementsDeclared: d.RequirementsFile != "", GitAutocrlf: d.GitAutocrlf}
		if !knownAutocrlf(out.GitAutocrlf) {
			out.GitAutocrlf = ""
			e.unavailable("deps", "dependencies.gitAutocrlf")
		}
		for _, req := range d.Requirements {
			if !knownDependency(req.Kind, req.Name) || !allowed(req.Status, "present", "missing", "unknown") {
				e.unavailable("deps", "dependencies.requirement")
				continue
			}
			out.Requirements = append(out.Requirements, Dependency{Kind: req.Kind, Name: req.Name, Status: req.Status, PathToken: e.paths.optional(req.Path)})
		}
		if p := d.Package; p != nil {
			out.Package = &PackageEvidence{ManifestPresent: p.Manifest != ""}
			for _, m := range []struct {
				raw      *collector.PackageManager
				dest     **PackageManager
				declared bool
			}{{p.Declared, &out.Package.Declared, true}, {p.Runner, &out.Package.Runner, false}} {
				if m.raw == nil {
					continue
				}
				if !knownPackageManager(m.raw.Name) || m.raw.Version != "" && !exactVersion(m.raw.Version) || m.declared && !exactVersion(m.raw.Version) {
					e.unavailable("deps", "dependencies.packageManager")
					continue
				}
				*m.dest = &PackageManager{Name: m.raw.Name, Version: m.raw.Version}
			}
			for _, lock := range p.Lockfiles {
				if knownLockfile(lock) {
					out.Package.Lockfiles = append(out.Package.Lockfiles, lock)
				} else {
					e.unavailable("deps", "dependencies.lockfile")
				}
			}
		}
		e.snapshot.Dependencies = out
		e.omit("dependencies.locations", "identifying_values")
	}
	if env := s.PythonEnvironment; env != nil {
		e.snapshot.PythonEnvironment = &PythonEnvironment{RootToken: e.environment(env.Root), CanonicalToken: e.environment(env.CanonicalRoot), MetadataKnown: env.MetadataKnown}
		e.omit("pythonEnvironment.locations", "identifying_values")
	}
	for _, p := range s.Proxies {
		parsed, err := url.Parse(p.URL)
		if err != nil || !allowed(p.Variable, "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY") || parsed == nil || !allowed(parsed.Scheme, "http", "https", "socks5", "socks5h") || p.Host == "" || p.Port < 1 || p.Port > 65535 || !allowed(p.Probe, "untested", "reachable", "unreachable", "unknown", "invalid") {
			e.unavailable("proxy", "proxy.endpoint")
			continue
		}
		e.snapshot.Proxies = append(e.snapshot.Proxies, Proxy{Variable: p.Variable, Scheme: parsed.Scheme, HostToken: e.hosts.token(p.Host) + ".invalid", Port: p.Port, Probe: p.Probe})
		e.omit("proxy.url", "identifying_values")
	}
	for _, c := range d.Collectors {
		if !knownCollector(c.Name) {
			return SharingSnapshot{}, report.Source{}, nil, Diagnostics{}, ErrEvidence
		}
		if c.DurationMs < 0 || c.DurationMs > maxDurationMs {
			return SharingSnapshot{}, report.Source{}, nil, Diagnostics{}, ErrBounds
		}
		if c.Error != "" {
			e.failed[c.Name] = true
			e.omit("collector.errors", "raw_error")
		}
	}
	for _, name := range collectorNames {
		if e.failed[name] {
			e.snapshot.Errors = append(e.snapshot.Errors, Failure{Collector: name, Reason: "unavailable"})
		}
	}
	timings := []collector.Timing{}
	diags := Diagnostics{SchemaVersion: 1, Collectors: []Diagnostic{}}
	seenTimings := map[string]bool{}
	for _, c := range d.Collectors {
		if seenTimings[c.Name] {
			return SharingSnapshot{}, report.Source{}, nil, Diagnostics{}, ErrEvidence
		}
		seenTimings[c.Name] = true
		timing := collector.Timing{Name: c.Name, Duration: time.Duration(c.DurationMs) * time.Millisecond}
		status := "available"
		if e.failed[c.Name] {
			status = "unavailable"
			timing.Err = fmt.Errorf("%s", unavailableNotice)
		}
		timings = append(timings, timing)
		diags.Collectors = append(diags.Collectors, Diagnostic{Collector: c.Name, DurationMs: c.DurationMs, Status: status})
	}
	for _, name := range collectorNames {
		if e.failed[name] && !seenTimings[name] {
			diags.Collectors = append(diags.Collectors, Diagnostic{Collector: name, Status: "unavailable"})
		}
	}
	normalizedVersion := toolVersion(d.Tool.Version)
	if normalizedVersion != d.Tool.Version {
		e.omit("tool.version", "unsupported_value")
	}
	source := report.Source{Kind: "bundle", CaptureTool: report.Tool{Name: "envdoctor", Version: normalizedVersion}, CapturedAt: e.snapshot.CollectedAt, RedactionPolicy: Policy, CaptureExitCode: d.ExitCode, Omissions: e.omissions}
	if len(source.Omissions) > 256 {
		return SharingSnapshot{}, report.Source{}, nil, Diagnostics{}, ErrBounds
	}
	return e.snapshot, source, timings, diags, nil
}

// engineSnapshot restores only fixed presence markers and opaque aliases. This
// conversion has no environment, filesystem, execution or network dependency.
func engineSnapshot(s SharingSnapshot) collector.Snapshot {
	out := collector.Snapshot{SchemaVersion: 1, CollectedAt: s.CollectedAt, RequestedPorts: append([]int(nil), s.RequestedPorts...), NoProxyConfigured: s.NoProxyConfigured}
	if p := s.Platform; p != nil {
		out.Platform = &collector.Platform{OS: p.OS, Version: p.Version, Arch: p.Arch, Elevated: p.Elevated}
	}
	if sh := s.Shell; sh != nil {
		out.Shell = &collector.Shell{Name: sh.Name}
	}
	if p := s.Path; p != nil {
		out.Path = &collector.PathInfo{Entries: make([]collector.PathEntry, 0, len(p.Entries))}
		for _, e := range p.Entries {
			out.Path.Entries = append(out.Path.Entries, collector.PathEntry{Raw: e.Token, Exists: e.Exists, IsDir: e.IsDir, Position: e.Position, Duplicate: e.Duplicate})
		}
	}
	for _, r := range s.Runtimes {
		out.Runtimes = append(out.Runtimes, collector.Runtime{Name: r.Name, Version: r.Version, Path: r.PathToken, Manager: r.Manager, Active: r.Active, Expected: r.Expected, Missing: r.Missing, ResolvedPath: r.ResolvedToken, Resolution: r.Resolution, EnvironmentRoot: r.EnvironmentToken})
	}
	for _, p := range s.Ports {
		out.Ports = append(out.Ports, collector.Port{Port: p.Port, Protocol: p.Protocol, State: p.State})
	}
	if d := s.Docker; d != nil {
		out.Docker = &collector.Docker{CLIVersion: d.CLIVersion, Installed: d.Installed, DaemonReachable: d.DaemonReachable, ProbeStatus: d.ProbeStatus, ClientAPI: d.ClientAPI, ClientMinAPI: d.ClientMinAPI, ClientDefaultAPI: d.ClientDefaultAPI, ServerAPI: d.ServerAPI, ServerMinAPI: d.ServerMinAPI, APIOverride: d.APIOverride}
		if d.Installed {
			out.Docker.Context = "local-default"
		}
	}
	if g := s.Git; g != nil {
		out.Git = &collector.Git{Version: g.Version, IdentityChecked: g.IdentityChecked, Autocrlf: g.Autocrlf}
		if g.UserNamePresent {
			marker := "present"
			out.Git.UserName = &marker
		}
		if g.UserEmailPresent {
			marker := "present"
			out.Git.UserEmail = &marker
		}
		if g.AutocrlfOriginKnown {
			out.Git.AutocrlfOrigin = "capture-origin"
		}
	}
	if d := s.Dependencies; d != nil {
		out.Dependencies = &collector.Dependencies{ProjectRoot: d.ProjectToken, GitAutocrlf: d.GitAutocrlf}
		if d.RequirementsDeclared {
			out.Dependencies.RequirementsFile = "capture-requirements"
		}
		for _, req := range d.Requirements {
			out.Dependencies.Requirements = append(out.Dependencies.Requirements, collector.Dependency{Kind: req.Kind, Name: req.Name, Path: req.PathToken, Status: req.Status})
		}
		if p := d.Package; p != nil {
			out.Dependencies.Package = &collector.PackageEvidence{Lockfiles: append([]string(nil), p.Lockfiles...)}
			if p.ManifestPresent {
				out.Dependencies.Package.Manifest = "capture-manifest"
			}
			if p.Declared != nil {
				out.Dependencies.Package.Declared = &collector.PackageManager{Name: p.Declared.Name, Version: p.Declared.Version}
			}
			if p.Runner != nil {
				out.Dependencies.Package.Runner = &collector.PackageManager{Name: p.Runner.Name, Version: p.Runner.Version}
			}
		}
	}
	if e := s.PythonEnvironment; e != nil {
		out.PythonEnvironment = &collector.PythonEnvironment{Root: e.RootToken, CanonicalRoot: e.CanonicalToken, MetadataKnown: e.MetadataKnown}
	}
	for _, p := range s.Proxies {
		out.Proxies = append(out.Proxies, collector.Proxy{Variable: p.Variable, URL: p.Scheme + "://" + net.JoinHostPort(p.HostToken, strconv.Itoa(p.Port)), Host: p.HostToken, Port: p.Port, Probe: p.Probe})
	}
	for _, failure := range s.Errors {
		out.Errors = append(out.Errors, collector.CollectorError{Collector: failure.Collector, Message: unavailableNotice})
	}
	return out
}

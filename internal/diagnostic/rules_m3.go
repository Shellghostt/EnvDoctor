package diagnostic

import (
	"path"
	"strconv"
	"strings"

	"github.com/shellghostt/envdoctor/internal/collector"
)

func dockerAPIMismatch(s collector.Snapshot) []Finding {
	d := s.Docker
	if d == nil || !d.Installed {
		return nil
	}
	serverMax, maxKnown := apiNumber(d.ServerAPI)
	serverMin, minKnown := apiNumber(d.ServerMinAPI)
	if !maxKnown || !minKnown || serverMin > serverMax {
		return nil
	}
	clientMaxText := d.ClientDefaultAPI
	if clientMaxText == "" {
		clientMaxText = d.ClientAPI
	}
	clientMax, clientKnown := apiNumber(clientMaxText)
	clientMin, clientMinKnown := apiNumber(d.ClientMinAPI)
	if clientMinKnown && clientKnown && clientMin > clientMax {
		return nil
	}
	incompatible := clientKnown && clientMax < serverMin || clientMinKnown && clientMin > serverMax
	evidence := []string{"Server API range: " + d.ServerMinAPI + " to " + d.ServerAPI}
	if clientKnown {
		evidence = append(evidence, "Client API maximum: "+clientMaxText)
	}
	if clientMinKnown {
		evidence = append(evidence, "Client API minimum: "+d.ClientMinAPI)
	}
	if d.APIOverride != "" {
		override, known := apiNumber(d.APIOverride)
		if !known {
			return nil
		}
		if override < serverMin || override > serverMax {
			incompatible = true
			evidence = append(evidence, "Explicit API override: "+d.APIOverride)
		}
	}
	if !incompatible {
		return nil
	}
	return []Finding{finding("ENV007", evidence)}
}

func apiNumber(value string) (int, bool) {
	major, minor, ok := strings.Cut(value, ".")
	if !ok || major != "1" || minor == "" || len(minor) > 6 {
		return 0, false
	}
	for _, ch := range minor {
		if ch < '0' || ch > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(minor)
	return n, err == nil
}

func missingDependencies(s collector.Snapshot) []Finding {
	d := s.Dependencies
	if d == nil || d.ProjectRoot == "" || d.RequirementsFile == "" {
		return nil
	}
	evidence := []string{}
	for _, req := range d.Requirements {
		if req.Status != "missing" {
			continue
		}
		if req.Kind == "tool" {
			switch req.Name {
			case "cmake", "make", "gcc", "pkg-config":
				evidence = append(evidence, "Required tool "+req.Name+": no usable executable on the inspected PATH")
			}
		} else if req.Kind == "library" && req.Name == "openssl" && req.Path != "" && s.Platform != nil && (s.Platform.OS == "linux" || s.Platform.OS == "darwin" || s.Platform.OS == "windows" && s.Platform.Arch == "amd64") {
			evidence = append(evidence, "Required library openssl: development files missing at "+req.Path)
		}
	}
	if len(evidence) == 0 {
		return nil
	}
	return []Finding{finding("ENV008", evidence)}
}

func packageManagerMismatch(s collector.Snapshot) []Finding {
	if s.Dependencies == nil || s.Dependencies.Package == nil {
		return nil
	}
	p := s.Dependencies.Package
	if p.Manifest == "" {
		return nil
	}
	families := map[string]bool{}
	evidence := []string{}
	for _, lock := range p.Lockfiles {
		family := lockfileFamily(lock)
		if family != "" {
			families[family] = true
			evidence = append(evidence, "Observed lockfile: "+lock)
		}
	}
	if len(families) == 0 {
		return nil
	}
	mismatch := len(families) > 1
	if mismatch {
		evidence = append(evidence, "Conflicting package-manager lockfile families")
	}
	declared, runner := p.Declared, p.Runner
	if declared != nil && (!supportedManager(declared.Name) || !exactVersion(declared.Version)) {
		declared = nil
	}
	if runner != nil && !supportedManager(runner.Name) {
		runner = nil
	}
	selected := declared
	if declared != nil {
		evidence = append(evidence, "Declared package manager: "+declared.Name+"@"+declared.Version)
	}
	if runner != nil {
		selected = runner
		text := runner.Name
		if exactVersion(runner.Version) {
			text += "@" + runner.Version
		}
		evidence = append(evidence, "Explicit runner: "+text)
	}
	if declared != nil && runner != nil && (declared.Name != runner.Name || exactVersion(runner.Version) && declared.Version != runner.Version) {
		mismatch = true
	}
	if selected != nil {
		for family := range families {
			if family != selected.Name {
				mismatch = true
			}
		}
	}
	if !mismatch {
		return nil
	}
	return []Finding{finding("ENV009", evidence)}
}

func supportedManager(name string) bool {
	return name == "npm" || name == "pnpm" || name == "yarn" || name == "bun"
}
func exactVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 9 || len(p) > 1 && p[0] == '0' {
			return false
		}
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	return true
}
func lockfileFamily(lock string) string {
	switch lock {
	case "package-lock.json", "npm-shrinkwrap.json":
		return "npm"
	case "pnpm-lock.yaml":
		return "pnpm"
	case "yarn.lock":
		return "yarn"
	case "bun.lock", "bun.lockb":
		return "bun"
	}
	return ""
}

func gitConfiguration(s collector.Snapshot) []Finding {
	g := s.Git
	if g == nil || !g.IdentityChecked || g.Version == "" {
		return nil
	}
	evidence := []string{}
	if g.UserName == nil || strings.TrimSpace(*g.UserName) == "" {
		evidence = append(evidence, "user.name is unset after effective config inspection")
	}
	if g.UserEmail == nil || strings.TrimSpace(*g.UserEmail) == "" {
		evidence = append(evidence, "user.email is unset after effective config inspection")
	}
	if d := s.Dependencies; d != nil && d.RequirementsFile != "" && (d.GitAutocrlf == "true" || d.GitAutocrlf == "false" || d.GitAutocrlf == "input") {
		actual, origin := g.Autocrlf, g.AutocrlfOrigin
		if actual == "" {
			actual, origin = "false", "default"
		}
		if origin != "" && (actual == "true" || actual == "false" || actual == "input") && actual != d.GitAutocrlf {
			evidence = append(evidence, "core.autocrlf="+actual+" ("+origin+") conflicts with declared policy "+d.GitAutocrlf+" ("+d.RequirementsFile+")")
		}
	}
	if len(evidence) == 0 {
		return nil
	}
	return []Finding{finding("ENV010", evidence)}
}

func virtualEnvironmentMismatch(s collector.Snapshot) []Finding {
	e := s.PythonEnvironment
	if e == nil || !e.MetadataKnown || e.CanonicalRoot == "" || s.Platform == nil || (s.Platform.OS != "windows" && s.Platform.OS != "linux" && s.Platform.OS != "darwin") {
		return nil
	}
	windows := s.Platform != nil && s.Platform.OS == "windows"
	expected := normalizeEnvironmentPath(e.CanonicalRoot, windows)
	evidence := []string{}
	for _, r := range s.Runtimes {
		if r.Name != "python" || !r.Active || r.Missing || r.Resolution != "known" || r.ResolvedPath == "" {
			continue
		}
		if r.EnvironmentRoot != "" && normalizeEnvironmentPath(r.EnvironmentRoot, windows) == expected {
			continue
		}
		evidence = append(evidence, "Selected Python invocation: "+r.Path)
	}
	if len(evidence) == 0 {
		return nil
	}
	evidence = append(evidence, "Active virtual environment: "+e.CanonicalRoot)
	return []Finding{finding("ENV011", evidence)}
}

func normalizeEnvironmentPath(value string, windows bool) string {
	value = path.Clean(strings.ReplaceAll(value, `\`, "/"))
	if windows {
		value = strings.ToLower(value)
	}
	return value
}

func proxyUnreachable(s collector.Snapshot) []Finding {
	evidence := []string{}
	for _, p := range s.Proxies {
		if p.Probe == "unreachable" && p.Host != "" && p.Port >= 1 && p.Port <= 65535 && (p.Variable == "HTTP_PROXY" || p.Variable == "HTTPS_PROXY" || p.Variable == "ALL_PROXY") {
			evidence = append(evidence, p.Variable+" "+p.URL+": connection refused or no route")
		}
	}
	if len(evidence) == 0 {
		return nil
	}
	return []Finding{finding("ENV012", evidence)}
}

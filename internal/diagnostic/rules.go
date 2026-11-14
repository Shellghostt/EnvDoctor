package diagnostic

import (
	"fmt"
	"github.com/shellghostt/envdoctor/internal/collector"
	"sort"
	"strconv"
	"strings"
)

type builtinRule struct {
	id, component string
	run           func(collector.Snapshot) []Finding
}

func (r builtinRule) ID() string           { return r.id }
func (r builtinRule) Components() []string { return []string{r.component} }
func (r builtinRule) Evaluate(s collector.Snapshot) []Finding {
	for _, e := range s.Errors {
		if e.Collector == r.component || (e.Collector == "runtimes" && (r.component == "python" || r.component == "node" || r.id == "ENV004")) {
			return nil
		}
	}
	return r.run(s)
}
func DefaultEngine() *Engine {
	return NewEngine(
		builtinRule{"ENV001", "python", pythonConflicts}, builtinRule{"ENV002", "node", nodeConflicts},
		builtinRule{"ENV003", "path", missingPath}, builtinRule{"ENV004", "path", shadowing},
		builtinRule{"ENV005", "network", portsInUse}, builtinRule{"ENV006", "docker", dockerUnavailable},
	)
}
func finding(id, component, title string, severity Severity, evidence []string, cause, fix string) Finding {
	sort.Strings(evidence)
	return Finding{RuleID: id, Component: component, Title: title, Severity: severity, Detected: evidence, Cause: cause, Fix: &Fix{Summary: fix, Automatic: false, Reversible: true}, Verify: "envdoctor diagnose --rule " + id, Docs: "https://github.com/shellghostt/envdoctor/blob/main/docs/rules/" + id + ".md"}
}
func pythonConflicts(s collector.Snapshot) []Finding {
	versions := map[string]bool{}
	evidence := []string{}
	for _, r := range s.Runtimes {
		if r.Name == "python" && r.Active && !r.Missing && r.Version != "" {
			versions[r.Version] = true
			evidence = append(evidence, r.Path+" ("+r.Version+")")
		}
	}
	if len(versions) < 2 {
		return nil
	}
	return []Finding{finding("ENV001", "python", "Conflicting active Python versions", SeverityError, evidence, "Active Python command aliases resolve to different installed versions.", "Activate the intended environment and align python and python3 on PATH.")}
}
func nodeConflicts(s collector.Snapshot) []Finding {
	paths := map[string]bool{}
	evidence := []string{}
	missing := false
	for _, r := range s.Runtimes {
		if r.Name != "node" {
			continue
		}
		if r.Missing && (r.Active || r.Expected) {
			missing = true
			evidence = append(evidence, r.Path+" (missing target)")
		} else if !r.Missing {
			paths[r.Path] = true
			evidence = append(evidence, r.Path)
		}
	}
	if len(paths) < 2 && !missing {
		return nil
	}
	return []Finding{finding("ENV002", "node", "Conflicting Node installations or missing target", SeverityError, evidence, "Multiple Node candidates or a selected manager target are inconsistent.", "Select one Node installation and repair the selected version-manager target.")}
}
func missingPath(s collector.Snapshot) []Finding {
	if s.Path == nil {
		return nil
	}
	evidence := []string{}
	for _, e := range s.Path.Entries {
		if !e.Exists {
			evidence = append(evidence, e.Raw)
		}
	}
	if len(evidence) == 0 {
		return nil
	}
	return []Finding{finding("ENV003", "path", "Missing PATH entries", SeverityWarning, evidence, "PATH references directories that no longer exist.", "Remove or correct these entries in your shell or system PATH settings.")}
}
func shadowing(s collector.Snapshot) []Finding {
	evidence := []string{}
	for _, expected := range s.Runtimes {
		if !expected.Expected || expected.Missing || expected.Active {
			continue
		}
		for _, active := range s.Runtimes {
			if active.Name == expected.Name && active.Active && !active.Missing && active.Path != expected.Path {
				evidence = append(evidence, active.Path+" shadows expected "+expected.Path)
			}
		}
	}
	if len(evidence) == 0 {
		return nil
	}
	return []Finding{finding("ENV004", "path", "Selected runtime is shadowed on PATH", SeverityWarning, evidence, "An earlier PATH candidate takes precedence over the selected environment.", "Activate the selected environment or move its bin directory before the conflicting entry.")}
}
func portsInUse(s collector.Snapshot) []Finding {
	evidence := []string{}
	ports := map[int]bool{}
	for _, p := range s.Ports {
		if p.State != "listen" || p.Protocol != "tcp" {
			continue
		}
		requested := false
		for _, port := range s.RequestedPorts {
			if port == p.Port {
				requested = true
			}
		}
		if !requested {
			continue
		}
		owner := "process unavailable"
		if p.PID > 0 {
			owner = fmt.Sprintf("pid %d", p.PID)
		}
		if p.Process != "" {
			owner = p.Process + " " + owner
		}
		evidence = append(evidence, fmt.Sprintf("TCP %d: %s", p.Port, strings.TrimSpace(owner)))
		ports[p.Port] = true
	}
	if len(evidence) == 0 {
		return nil
	}
	result := finding("ENV005", "network", "Requested TCP port is already in use", SeverityError, evidence, "A listening process already holds a port requested for this application.", "Choose another port or stop the identified process after confirming it is safe to stop.")
	ordered := make([]int, 0, len(ports))
	for port := range ports {
		ordered = append(ordered, port)
	}
	sort.Ints(ordered)
	values := make([]string, 0, len(ordered))
	for _, port := range ordered {
		values = append(values, strconv.Itoa(port))
	}
	result.Verify += " --port " + strings.Join(values, ",")
	return []Finding{result}
}
func dockerUnavailable(s collector.Snapshot) []Finding {
	if s.Docker == nil || !s.Docker.Installed || s.Docker.DaemonReachable {
		return nil
	}
	return []Finding{finding("ENV006", "docker", "Docker daemon unavailable", SeverityError, []string{"Docker CLI installed (" + s.Docker.CLIVersion + ")", "Local default Docker daemon did not answer"}, "The local default Docker engine is stopped, inaccessible, or uses another socket.", "Start the local Docker engine and check permission to its default socket.")}
}

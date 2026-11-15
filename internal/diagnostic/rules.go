package diagnostic

import (
	"fmt"
	"github.com/shellghostt/envdoctor/internal/collector"
	"sort"
	"strconv"
	"strings"
)

type builtinRule struct {
	metadata RuleMetadata
	run      func(collector.Snapshot) []Finding
}

func (r builtinRule) ID() string           { return r.metadata.RuleID }
func (r builtinRule) Components() []string { return []string{r.metadata.Component} }
func (r builtinRule) Evaluate(s collector.Snapshot) []Finding {
	for _, e := range s.Errors {
		for _, prerequisite := range r.metadata.Prerequisites {
			if e.Collector == prerequisite {
				return nil
			}
		}
	}
	return r.run(s)
}
func DefaultEngine() *Engine {
	rules := make([]Rule, 0, len(builtinRules))
	for _, rule := range builtinRules {
		rules = append(rules, rule)
	}
	return NewEngine(rules...)
}
func finding(id string, evidence []string) Finding {
	metadata, _ := LookupRule(id)
	sort.Strings(evidence)
	return Finding{RuleID: id, Component: metadata.Component, Title: metadata.Title, Severity: metadata.Severity, Detected: evidence, Cause: metadata.Explanation, Fix: &Fix{Summary: metadata.ManualSteps[0], Automatic: false, Reversible: true}, Verify: metadata.Verify, Docs: metadata.Docs}
}
func pythonConflicts(s collector.Snapshot) []Finding {
	versions := map[string]bool{}
	evidence := []string{}
	for _, r := range s.Runtimes {
		if r.Name == "python" && r.Active && !r.Missing && r.Version != "" && r.Resolution != "unknown" {
			versions[r.Version] = true
			evidence = append(evidence, r.Path+" ("+r.Version+")")
		}
	}
	if len(versions) < 2 {
		return nil
	}
	return []Finding{finding("ENV001", evidence)}
}
func nodeConflicts(s collector.Snapshot) []Finding {
	paths := map[string]bool{}
	evidence := []string{}
	missing := false
	for _, r := range s.Runtimes {
		if r.Name != "node" {
			continue
		}
		if r.Resolution == "unknown" {
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
	return []Finding{finding("ENV002", evidence)}
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
	return []Finding{finding("ENV003", evidence)}
}
func shadowing(s collector.Snapshot) []Finding {
	evidence := []string{}
	for _, expected := range s.Runtimes {
		if !expected.Expected || expected.Missing || expected.Active || expected.Resolution == "unknown" {
			continue
		}
		for _, active := range s.Runtimes {
			if active.Name == expected.Name && active.Active && !active.Missing && active.Path != expected.Path && active.Resolution != "unknown" {
				evidence = append(evidence, active.Path+" shadows expected "+expected.Path)
			}
		}
	}
	if len(evidence) == 0 {
		return nil
	}
	return []Finding{finding("ENV004", evidence)}
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
	result := finding("ENV005", evidence)
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
	if s.Docker.ProbeStatus != "" && s.Docker.ProbeStatus != "unavailable" {
		return nil
	}
	return []Finding{finding("ENV006", []string{"Docker CLI installed (" + s.Docker.CLIVersion + ")", "Local default Docker daemon did not answer"})}
}

package bundle

import (
	"reflect"
	"strings"
	"sync"

	"github.com/shellghostt/envdoctor/internal/collector"
	"github.com/shellghostt/envdoctor/internal/report"
)

// A new source-model field requires an explicit sharing decision. Separate
// output types prevent raw copying; this guard also prevents silently ignoring
// a future fact that might be a new prerequisite. Order is intentionally exact.
var currentSourceModel = sync.OnceValue(func() bool {
	for typ, names := range map[reflect.Type]string{
		reflect.TypeFor[collector.Snapshot]():          "SchemaVersion CollectedAt Platform Shell Path Runtimes Ports RequestedPorts Docker Git Dependencies PythonEnvironment Proxies NoProxyConfigured Errors",
		reflect.TypeFor[collector.Platform]():          "OS Version Arch Elevated",
		reflect.TypeFor[collector.Shell]():             "Name Path",
		reflect.TypeFor[collector.PathInfo]():          "Entries",
		reflect.TypeFor[collector.PathEntry]():         "Raw Exists IsDir Position Duplicate",
		reflect.TypeFor[collector.Runtime]():           "Name Version Path Manager Active Expected Missing ResolvedPath Resolution EnvironmentRoot",
		reflect.TypeFor[collector.Port]():              "Port Protocol State PID Process",
		reflect.TypeFor[collector.Docker]():            "CLIVersion DaemonReachable Context Installed ProbeStatus ClientAPI ClientMinAPI ClientDefaultAPI ServerAPI ServerMinAPI APIOverride SelectedContext SelectedHost",
		reflect.TypeFor[collector.Git]():               "Version UserName UserEmail Autocrlf IdentityChecked UserNameOrigin UserEmailOrigin AutocrlfOrigin",
		reflect.TypeFor[collector.Dependencies]():      "ProjectRoot RequirementsFile Requirements Package GitAutocrlf",
		reflect.TypeFor[collector.Dependency]():        "Kind Name Path Status",
		reflect.TypeFor[collector.PackageManager]():    "Name Version",
		reflect.TypeFor[collector.PackageEvidence]():   "Manifest Declared Runner Lockfiles",
		reflect.TypeFor[collector.PythonEnvironment](): "Root CanonicalRoot MetadataKnown",
		reflect.TypeFor[collector.Proxy]():             "Variable URL Host Port Probe",
		reflect.TypeFor[collector.CollectorError]():    "Collector Message",
		reflect.TypeFor[report.Tool]():                 "Name Version",
		reflect.TypeFor[report.Collector]():            "Name DurationMs Error",
	} {
		fields := strings.Fields(names)
		if typ.NumField() != len(fields) {
			return false
		}
		for i, name := range fields {
			if typ.Field(i).Name != name {
				return false
			}
		}
	}
	return true
})

package collector

import "github.com/shellghostt/envdoctor/internal/system"

// The two indirections below keep collectors free of direct operating-system
// calls. Tests replace the fields on the collector value instead.
func platformInfo() (string, bool) { return system.PlatformInfo() }

func shellInfo() (string, string) { return system.ShellInfo() }

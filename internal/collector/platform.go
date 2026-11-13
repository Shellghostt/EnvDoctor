// Package collector: platform and shell collectors.
package collector

import (
	"context"
	"runtime"
)

// PlatformCollector reads OS, version, architecture and elevation.
type PlatformCollector struct {
	OS   string
	Arch string
	Info func() (version string, elevated bool)
}

// NewPlatformCollector binds the collector to the running process.
func NewPlatformCollector() PlatformCollector {
	return PlatformCollector{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		Info: platformInfo,
	}
}

// Name implements Collector.
func (c PlatformCollector) Name() string { return "platform" }

// Collect implements Collector. The version may be empty when the OS does not
// expose it; that is reported as empty rather than invented.
func (c PlatformCollector) Collect(_ context.Context) (Part, error) {
	version, elevated := c.Info()
	return Part{
		Platform: &Platform{
			OS:       c.OS,
			Version:  version,
			Arch:     c.Arch,
			Elevated: elevated,
		},
	}, nil
}

// ShellCollector reads the shell that launched the process.
type ShellCollector struct {
	Info func() (name, path string)
}

// NewShellCollector binds the collector to the running process.
func NewShellCollector() ShellCollector {
	return ShellCollector{Info: shellInfo}
}

// Name implements Collector.
func (c ShellCollector) Name() string { return "shell" }

// Collect implements Collector.
func (c ShellCollector) Collect(_ context.Context) (Part, error) {
	name, path := c.Info()
	return Part{Shell: &Shell{Name: name, Path: path}}, nil
}

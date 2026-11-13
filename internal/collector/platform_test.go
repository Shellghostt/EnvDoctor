package collector_test

import (
	"context"
	"testing"

	"github.com/shellghostt/envdoctor/internal/collector"
)

func TestPlatformCollectorUsesInjectedInfo(t *testing.T) {
	c := collector.PlatformCollector{
		OS:   "linux",
		Arch: "arm64",
		Info: func() (string, bool) { return "Ubuntu 24.04", true },
	}
	part, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if part.Platform == nil {
		t.Fatal("platform part missing")
	}

	got := *part.Platform
	if got.OS != "linux" || got.Arch != "arm64" || got.Version != "Ubuntu 24.04" || !got.Elevated {
		t.Fatalf("platform = %+v", got)
	}
}

func TestPlatformCollectorKeepsUnknownVersionEmpty(t *testing.T) {
	c := collector.PlatformCollector{
		OS:   "linux",
		Arch: "amd64",
		Info: func() (string, bool) { return "", false },
	}
	part, _ := c.Collect(context.Background())

	if part.Platform.Version != "" {
		t.Fatalf("version was invented: %q", part.Platform.Version)
	}
}

func TestShellCollectorUsesInjectedInfo(t *testing.T) {
	c := collector.ShellCollector{
		Info: func() (string, string) { return "zsh", "/usr/bin/zsh" },
	}
	part, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if part.Shell == nil || part.Shell.Name != "zsh" || part.Shell.Path != "/usr/bin/zsh" {
		t.Fatalf("shell = %+v", part.Shell)
	}
}

func TestShellCollectorAllowsUnknownPath(t *testing.T) {
	c := collector.ShellCollector{Info: func() (string, string) { return "powershell", "" }}
	part, _ := c.Collect(context.Background())

	if part.Shell.Name != "powershell" || part.Shell.Path != "" {
		t.Fatalf("shell = %+v", part.Shell)
	}
}

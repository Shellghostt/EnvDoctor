package collector

import (
	"context"
	"github.com/shellghostt/envdoctor/internal/system"
	"strings"
)

type GitCollector struct {
	FindTool func(string) string
	RunTool  func(context.Context, string, ...string) ([]byte, error)
}

func (GitCollector) Name() string { return "git" }
func (c GitCollector) Collect(ctx context.Context) (Part, error) {
	find, run := c.FindTool, c.RunTool
	if find == nil {
		find = system.InstalledTool
	}
	if run == nil {
		run = system.RunTool
	}
	path := find("git")
	if path == "" {
		return Part{}, nil
	}
	version, err := run(ctx, path, "--version")
	if err != nil {
		return Part{}, err
	}
	g := &Git{Version: strings.TrimSpace(strings.TrimPrefix(string(version), "git version "))}
	for _, key := range []string{"user.name", "user.email", "core.autocrlf"} {
		out, err := run(ctx, path, "config", "--get", key)
		if ctx.Err() != nil {
			return Part{}, ctx.Err()
		}
		if err != nil {
			if system.ExitedWith(err, 1) {
				continue // Missing config values are different from malformed config.
			}
			return Part{}, err
		}
		value := strings.TrimSpace(string(out))
		switch key {
		case "user.name":
			g.UserName = &value
		case "user.email":
			g.UserEmail = &value
		case "core.autocrlf":
			g.Autocrlf = value
		}
	}
	return Part{Git: g}, nil
}

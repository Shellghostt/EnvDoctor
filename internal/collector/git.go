package collector

import (
	"context"
	"fmt"
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
	g := &Git{Version: strings.TrimSpace(strings.TrimPrefix(string(version), "git version ")), IdentityChecked: true}
	for _, key := range []string{"user.name", "user.email", "core.autocrlf"} {
		args := []string{"config", "--null", "--show-origin", "--get", key}
		out, err := run(ctx, path, args...)
		if ctx.Err() != nil {
			return Part{}, ctx.Err()
		}
		if err != nil {
			if system.ExitedWith(err, 1) {
				continue // Missing config values are different from malformed config.
			}
			return Part{}, err
		}
		fields := strings.Split(string(out), "\x00")
		if len(fields) != 3 || fields[0] == "" || fields[2] != "" {
			return Part{}, fmt.Errorf("Git config returned invalid origin metadata")
		}
		origin, value := fields[0], strings.TrimSpace(fields[1])
		// Older Git versions lack bool-or-str. Read the input mode literally,
		// otherwise use the longstanding bool type to canonicalize aliases.
		if key == "core.autocrlf" && value != "input" {
			out, err = run(ctx, path, "config", "--type=bool", "--null", "--show-origin", "--get", "core.autocrlf")
			if ctx.Err() != nil {
				return Part{}, ctx.Err()
			}
			if err != nil {
				return Part{}, err
			}
			fields = strings.Split(string(out), "\x00")
			if len(fields) != 3 || fields[0] == "" || fields[2] != "" {
				return Part{}, fmt.Errorf("Git config returned invalid boolean origin metadata")
			}
			origin, value = fields[0], strings.TrimSpace(fields[1])
		}
		switch key {
		case "user.name":
			g.UserName = &value
			g.UserNameOrigin = origin
		case "user.email":
			g.UserEmail = &value
			g.UserEmailOrigin = origin
		case "core.autocrlf":
			if value != "true" && value != "false" && value != "input" {
				return Part{}, fmt.Errorf("Git core.autocrlf is unsupported or invalid")
			}
			g.Autocrlf = value
			g.AutocrlfOrigin = origin
		}
	}
	return Part{Git: g}, nil
}

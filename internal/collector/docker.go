package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/shellghostt/envdoctor/internal/system"
	"runtime"
)

type DockerCollector struct {
	FindTool func(string) string
	RunTool  func(context.Context, string, ...string) ([]byte, error)
}

func (DockerCollector) Name() string { return "docker" }
func (c DockerCollector) Collect(ctx context.Context) (Part, error) {
	find, run := c.FindTool, c.RunTool
	if find == nil {
		find = system.InstalledTool
	}
	if run == nil {
		run = system.RunTool
	}
	path := find("docker")
	if path == "" {
		return Part{Docker: &Docker{}}, nil
	}
	host := "unix:///var/run/docker.sock"
	if runtime.GOOS == "windows" {
		host = "npipe:////./pipe/docker_engine"
	}
	out, err := run(ctx, path, "--host", host, "version", "--format", "{{json .}}")
	if ctx.Err() != nil {
		return Part{}, ctx.Err()
	}
	var version struct {
		Client *struct{ Version string }
		Server *struct{ Version string }
	}
	if json.Unmarshal(out, &version) != nil || version.Client == nil {
		if ctx.Err() != nil {
			return Part{}, ctx.Err()
		}
		if err != nil {
			return Part{}, err
		}
		return Part{}, fmt.Errorf("Docker version returned invalid metadata")
	}
	d := &Docker{Installed: true, Context: "local-default", DaemonReachable: err == nil && version.Server != nil}
	if version.Client != nil {
		d.CLIVersion = version.Client.Version
	}
	return Part{Docker: d}, nil
}

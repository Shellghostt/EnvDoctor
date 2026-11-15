package collector

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/shellghostt/envdoctor/internal/system"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type DockerCollector struct {
	FindTool func(string) string
	RunTool  func(context.Context, string, ...string) ([]byte, error)
	Getenv   func(string) string
	HomeDir  func() (string, error)
	ReadFile func(string) ([]byte, error)
	GOOS     string
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
		return Part{Docker: &Docker{ProbeStatus: "absent"}}, nil
	}
	getenv := c.Getenv
	if getenv == nil {
		getenv = system.Getenv
	}
	goos := c.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	d := &Docker{Installed: true, Context: "local-default", ProbeStatus: "unknown", APIOverride: getenv("DOCKER_API_VERSION")}
	part := Part{Docker: d}
	if err := c.contextMetadata(d, getenv); err != nil {
		part.Errors = append(part.Errors, CollectorError{Collector: "docker", Message: "selected Docker context metadata could not be inspected"})
	}
	host := "unix:///var/run/docker.sock"
	if goos == "windows" {
		host = "npipe:////./pipe/docker_engine"
	}
	out, err := run(ctx, path, "--host", host, "version", "--format", "{{json .}}")
	if ctx.Err() != nil {
		return Part{}, ctx.Err()
	}
	var version struct {
		Client *struct{ Version, ApiVersion, MinAPIVersion, DefaultAPIVersion string }
		Server *struct{ Version, ApiVersion, MinAPIVersion string }
	}
	if json.Unmarshal(out, &version) != nil || version.Client == nil {
		if ctx.Err() != nil {
			return Part{}, ctx.Err()
		}
		part.Errors = append(part.Errors, CollectorError{Collector: "docker", Message: "Docker version returned invalid or unavailable metadata"})
		return part, nil
	}
	d.CLIVersion = version.Client.Version
	d.ClientAPI, d.ClientMinAPI, d.ClientDefaultAPI = version.Client.ApiVersion, version.Client.MinAPIVersion, version.Client.DefaultAPIVersion
	if version.Server != nil {
		d.ServerAPI, d.ServerMinAPI = version.Server.ApiVersion, version.Server.MinAPIVersion
	}
	if err == nil && version.Server != nil {
		d.DaemonReachable, d.ProbeStatus = true, "available"
	} else {
		var toolError *system.ToolError
		detail := ""
		if errors.As(err, &toolError) {
			detail = strings.ToLower(toolError.Detail)
		}
		switch {
		case strings.Contains(detail, "permission denied"), strings.Contains(detail, "access is denied"), strings.Contains(detail, "access denied"):
			d.ProbeStatus = "denied"
		case strings.Contains(detail, "minimum supported api"), strings.Contains(detail, "client version") && (strings.Contains(detail, "too old") || strings.Contains(detail, "too new")):
			d.ProbeStatus = "unsupported"
		case strings.Contains(detail, "cannot connect to the docker daemon"), strings.Contains(detail, "is the docker daemon running"), strings.Contains(detail, "connection refused"), strings.Contains(detail, "no such file or directory"), strings.Contains(detail, "the system cannot find the file specified"):
			d.ProbeStatus = "unavailable"
		}
		if d.ProbeStatus != "unavailable" {
			part.Errors = append(part.Errors, CollectorError{Collector: "docker", Message: "local Docker inspection is " + d.ProbeStatus + "; daemon availability or API bounds are inconclusive"})
		}
	}
	return part, nil
}

// Context selection is data only. Never pass this endpoint or an environment
// context name to the CLI; the actual probe above always uses the local default.
func (c DockerCollector) contextMetadata(d *Docker, getenv func(string) string) error {
	read, homeDir := c.ReadFile, c.HomeDir
	if read == nil {
		read = system.ReadSmallFile
	}
	if homeDir == nil {
		homeDir = system.HomeDir
	}
	if getenv("DOCKER_CONFIG") != "" {
		return fmt.Errorf("alternate Docker configuration directory is unsupported")
	}
	if name := getenv("DOCKER_CONTEXT"); name != "" {
		d.SelectedContext = name
	} else if host := getenv("DOCKER_HOST"); host != "" {
		d.SelectedContext, d.SelectedHost = "environment", host
		return nil
	}
	home, err := homeDir()
	if err != nil {
		return err
	}
	if d.SelectedContext == "" {
		data, readErr := read(filepath.Join(home, ".docker", "config.json"))
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return readErr
		}
		if readErr == nil {
			var config struct {
				CurrentContext string `json:"currentContext"`
			}
			if len(data) > 1<<20 || json.Unmarshal(data, &config) != nil {
				return fmt.Errorf("invalid Docker context configuration")
			}
			d.SelectedContext = config.CurrentContext
		}
	}
	if d.SelectedContext == "" || d.SelectedContext == "default" {
		d.SelectedContext = "default"
		return nil
	}
	if len(d.SelectedContext) > 256 {
		return fmt.Errorf("Docker context name too long")
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(d.SelectedContext)))
	data, err := read(filepath.Join(home, ".docker", "contexts", "meta", digest, "meta.json"))
	if err != nil {
		return err
	}
	var metadata struct {
		Name      string
		Endpoints struct{ Docker struct{ Host string } }
	}
	if len(data) > 1<<20 || json.Unmarshal(data, &metadata) != nil || metadata.Name != d.SelectedContext {
		return fmt.Errorf("invalid Docker context metadata")
	}
	d.SelectedHost = metadata.Endpoints.Docker.Host
	return nil
}

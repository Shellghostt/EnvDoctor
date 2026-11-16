package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shellghostt/envdoctor/internal/config"
	"github.com/shellghostt/envdoctor/internal/system"
)

const RequirementsFile = "envdoctor.requirements.json"

type projectRequirement struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
}
type projectRequirements struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Requirements  []projectRequirement `json:"requirements,omitempty"`
	GitAutocrlf   string               `json:"gitAutocrlf,omitempty"`
}

type DependencyCollector struct {
	PackageManager string
	WorkingDir     func() (string, error)
	Getenv         func(string) string
	ReadFile       func(string) ([]byte, error)
	Stat           func(string) (os.FileInfo, error)
	GOOS, Arch     string
}

func (DependencyCollector) Name() string { return "deps" }

func (c DependencyCollector) Collect(ctx context.Context) (Part, error) {
	if err := ctx.Err(); err != nil {
		return Part{}, err
	}
	if c.WorkingDir == nil {
		c.WorkingDir = system.WorkingDir
	}
	if c.Getenv == nil {
		c.Getenv = system.Getenv
	}
	if c.ReadFile == nil {
		c.ReadFile = system.ReadSmallFile
	}
	if c.Stat == nil {
		c.Stat = system.Stat
	}
	if c.GOOS == "" {
		c.GOOS = runtime.GOOS
	}
	if c.Arch == "" {
		c.Arch = runtime.GOARCH
	}
	root, err := c.WorkingDir()
	if err != nil {
		return Part{}, err
	}
	d := &Dependencies{ProjectRoot: root}
	part := Part{Dependencies: d}
	data, found, err := c.readOptional(filepath.Join(root, RequirementsFile))
	if err != nil {
		return Part{}, fmt.Errorf("read project requirements: %w", err)
	}
	if found {
		var manifest projectRequirements
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&manifest) != nil || decoder.Decode(new(any)) != io.EOF || manifest.SchemaVersion != 1 || len(manifest.Requirements) > 32 {
			return Part{}, fmt.Errorf("invalid project requirements: expected one version-1 document with at most 32 requirements")
		}
		if manifest.GitAutocrlf != "" && manifest.GitAutocrlf != "true" && manifest.GitAutocrlf != "false" && manifest.GitAutocrlf != "input" {
			return Part{}, fmt.Errorf("invalid project gitAutocrlf policy")
		}
		d.RequirementsFile, d.GitAutocrlf = filepath.Join(root, RequirementsFile), manifest.GitAutocrlf
		seen := map[string]bool{}
		for _, req := range manifest.Requirements {
			if (req.Kind != "tool" && req.Kind != "library") || req.Name == "" || len(req.Name) > 64 || len(req.Path) > 4096 {
				return Part{}, fmt.Errorf("invalid project requirement kind, name or path")
			}
			if err := ctx.Err(); err != nil {
				return Part{}, err
			}
			key := req.Kind + "|" + req.Name + "|" + req.Path
			if seen[key] {
				continue
			}
			seen[key] = true
			dependency, inspectErr := c.inspectRequirement(ctx, root, req)
			d.Requirements = append(d.Requirements, dependency)
			if inspectErr != nil {
				part.Errors = append(part.Errors, CollectorError{Collector: "deps", Message: "dependency metadata is unknown (inspection failed or was denied)"})
			}
		}
	}
	data, found, err = c.readOptional(filepath.Join(root, "package.json"))
	if err != nil {
		return Part{}, fmt.Errorf("read package manifest: %w", err)
	}
	if found {
		var manifest struct {
			PackageManager string `json:"packageManager"`
		}
		if json.Unmarshal(data, &manifest) != nil {
			return Part{}, fmt.Errorf("invalid package.json metadata")
		}
		p := &PackageEvidence{Manifest: filepath.Join(root, "package.json")}
		name, version, parseErr := config.ParsePackageManager(manifest.PackageManager)
		if parseErr == nil && version != "" {
			p.Declared = &PackageManager{Name: name, Version: version}
		}
		if c.PackageManager != "" {
			name, version, parseErr = config.ParsePackageManager(c.PackageManager)
			if parseErr != nil {
				return Part{}, parseErr
			}
			p.Runner = &PackageManager{Name: name, Version: version}
		}
		for _, filename := range []string{"package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb"} {
			if err := ctx.Err(); err != nil {
				return Part{}, err
			}
			info, statErr := c.Stat(filepath.Join(root, filename))
			if statErr == nil && info.Mode().IsRegular() {
				p.Lockfiles = append(p.Lockfiles, filename)
			} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				return Part{}, fmt.Errorf("inspect project lockfile: %w", statErr)
			}
		}
		d.Package = p
	}
	return part, nil
}

func (c DependencyCollector) readOptional(path string) ([]byte, bool, error) {
	data, err := c.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(data) > 1<<20 {
		return nil, false, fmt.Errorf("project metadata exceeds 1 MiB")
	}
	return data, true, nil
}

func (c DependencyCollector) inspectRequirement(ctx context.Context, root string, req projectRequirement) (Dependency, error) {
	d := Dependency{Kind: req.Kind, Name: req.Name, Status: "unknown"}
	if req.Kind == "tool" && req.Path == "" {
		switch req.Name {
		case "cmake", "make", "gcc", "pkg-config":
		default:
			return d, nil
		}
		names := []string{req.Name}
		sep := ":"
		if c.GOOS == "windows" {
			sep = ";"
			names = []string{req.Name + ".exe"}
			if req.Name == "make" {
				names = append(names, "mingw32-make.exe")
			}
		}
		ambiguous := false
		var inspectionErr error
		for _, dir := range strings.Split(c.Getenv("PATH"), sep) {
			if err := ctx.Err(); err != nil {
				return d, err
			}
			dir = strings.Trim(dir, `"`)
			if dir == "" {
				ambiguous = true // Empty PATH components may select the current directory.
				continue
			}
			if !filepath.IsAbs(dir) {
				ambiguous = true
				continue
			}
			for _, name := range names {
				path := filepath.Join(dir, name)
				info, err := c.Stat(path)
				if err == nil && info.Mode().IsRegular() && (c.GOOS == "windows" || info.Mode()&0111 != 0) {
					d.Path, d.Status = path, "present"
					return d, nil
				}
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					inspectionErr = err
				}
			}
		}
		if inspectionErr != nil {
			return d, inspectionErr
		}
		if !ambiguous {
			d.Status = "missing"
		}
		return d, nil
	}
	if req.Kind != "library" || req.Name != "openssl" || req.Path == "" || c.GOOS == "windows" && c.Arch != "amd64" {
		return d, nil
	}
	prefix := req.Path
	if !filepath.IsAbs(prefix) {
		prefix = filepath.Join(root, prefix)
	}
	d.Path = filepath.Clean(prefix)
	header, err := c.Stat(filepath.Join(prefix, "include", "openssl", "ssl.h"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return d, err
	}
	if errors.Is(err, os.ErrNotExist) || err == nil && !header.Mode().IsRegular() {
		d.Status = "missing"
		return d, nil
	}
	dirs, extensions := []string{"lib"}, []string{".so", ".a"}
	if c.GOOS == "linux" {
		dirs = append(dirs, "lib64")
	}
	if c.GOOS == "darwin" {
		extensions = []string{".dylib", ".a"}
	}
	if c.GOOS == "windows" {
		dirs = append(dirs, filepath.Join("lib", "VC", "x64", "MD"))
		extensions = []string{".lib"}
	}
	var inspectionErr error
	for _, dir := range dirs {
		complete := true
		for _, name := range []string{"libssl", "libcrypto"} {
			present := false
			for _, extension := range extensions {
				if err := ctx.Err(); err != nil {
					return d, err
				}
				info, statErr := c.Stat(filepath.Join(prefix, dir, name+extension))
				if statErr == nil && info.Mode().IsRegular() {
					present = true
					break
				}
				if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
					inspectionErr = statErr
				}
			}
			complete = complete && present
		}
		if complete {
			d.Status = "present"
			return d, nil
		}
	}
	if inspectionErr != nil {
		return d, inspectionErr
	}
	d.Status = "missing"
	return d, nil
}

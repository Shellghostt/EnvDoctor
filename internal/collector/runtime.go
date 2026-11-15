package collector

import (
	"context"
	"errors"
	"github.com/shellghostt/envdoctor/internal/system"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// RuntimeCollector inspects PATH metadata and selected manager locations. It never
// executes a discovered interpreter, shim or script. Unknown versions stay empty.
type RuntimeCollector struct {
	Getenv func(string) string
	GOOS   string
}

func NewRuntimeCollector() RuntimeCollector { return RuntimeCollector{os.Getenv, runtime.GOOS} }
func (RuntimeCollector) Name() string       { return "runtimes" }

var pathVersion = regexp.MustCompile(`(?:^|[/\\])v?([0-9]+\.[0-9]+(?:\.[0-9]+)?)(?:[/\\]|$)`)
var numericVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+(?:\.[0-9]+)?$`)

func (c RuntimeCollector) Collect(ctx context.Context) (Part, error) {
	getenv := c.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	goos := c.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	aliases := []string{"python", "python3", "node"}
	suffix := ""
	sep := ":"
	if goos == "windows" {
		suffix = ".exe"
		sep = ";"
	}
	out := []Runtime{}
	seen := map[string]int{}
	active := map[string]bool{}
	var inspectionErr error
	expected := map[string]string{}
	var environment *PythonEnvironment
	if v := getenv("VIRTUAL_ENV"); v != "" {
		environment = &PythonEnvironment{Root: v}
		if filepath.IsAbs(v) {
			data, readErr := system.ReadSmallFile(filepath.Join(v, "pyvenv.cfg"))
			canonical, resolveErr := system.ResolvePath(v)
			if readErr == nil && resolveErr == nil && knownVenvMetadata(data) {
				environment.CanonicalRoot, environment.MetadataKnown = canonical, true
			}
		}
		sub := "bin"
		if goos == "windows" {
			sub = "Scripts"
		}
		expected["python"] = filepath.Join(v, sub, "python"+suffix)
	}
	if v := getenv("NVM_BIN"); v != "" {
		expected["node"] = filepath.Join(v, "node"+suffix)
	} else if v := getenv("NVM_SYMLINK"); v != "" {
		expected["node"] = filepath.Join(v, "node"+suffix)
	}
	add := func(path, name string, isActive, isExpected bool) {
		packaged := packagedRuntimePath(path, goos)
		resolved := path
		var err error
		if !packaged {
			resolved, err = filepath.EvalSymlinks(path)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			inspectionErr = err
		}
		missing := errors.Is(err, os.ErrNotExist)
		if err != nil {
			resolved = path
		}
		venvRoot := filepath.Dir(filepath.Dir(path))
		var venvData []byte
		if name == "python" && !packaged {
			var readErr error
			venvData, readErr = system.ReadSmallFile(filepath.Join(venvRoot, "pyvenv.cfg"))
			if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
				inspectionErr = readErr
			}
		}
		key := name + "|" + filepath.Clean(resolved)
		// A venv may symlink to the system interpreter; its invocation directory
		// still changes Python's environment. Deduplicate aliases within that venv.
		if len(venvData) > 0 {
			canonicalRoot, rootErr := system.ResolvePath(venvRoot)
			if rootErr != nil {
				canonicalRoot = venvRoot
			}
			key += "|" + canonicalRoot
		}
		if goos == "windows" {
			key = strings.ToLower(key)
		}
		if i, ok := seen[key]; ok {
			out[i].Active = out[i].Active || isActive
			out[i].Expected = out[i].Expected || isExpected
			return
		}
		manager := "system"
		lower := strings.ToLower(filepath.ToSlash(path) + "|" + filepath.ToSlash(resolved))
		for _, m := range []string{"pyenv", "nvm", "fnm", "volta"} {
			if strings.Contains(lower, "."+m+"/") || strings.Contains(lower, "/"+m+"/") {
				manager = m
				break
			}
		}
		if len(venvData) > 0 {
			manager = "venv"
		}
		version := ""
		if manager != "system" && manager != "venv" {
			if matches := pathVersion.FindAllStringSubmatch(resolved, -1); len(matches) > 0 {
				version = matches[len(matches)-1][1]
			}
		}
		if version == "" && !missing && !packaged {
			version = system.FileVersion(resolved)
		}
		if version == "" && len(venvData) > 0 {
			for _, line := range strings.Split(string(venvData), "\n") {
				k, v, ok := strings.Cut(line, "=")
				if ok && strings.TrimSpace(k) == "version" && numericVersion.MatchString(strings.TrimSpace(v)) {
					version = strings.TrimSpace(v)
				}
			}
		}
		resolution := "known"
		if missing || err != nil || packaged || strings.Contains(lower, "/shims/") {
			resolution = "unknown"
		}
		if resolution == "known" && name == "python" {
			native, nativeErr := system.NativeExecutable(resolved, goos)
			if !native {
				resolution = "unknown"
			}
			if nativeErr != nil {
				inspectionErr = nativeErr
			}
		}
		environmentRoot := ""
		if knownVenvMetadata(venvData) {
			environmentRoot, _ = system.ResolvePath(venvRoot)
			if environmentRoot == "" {
				resolution = "unknown"
			}
		}
		seen[key] = len(out)
		out = append(out, Runtime{Name: name, Version: version, Path: path, Manager: manager, Active: isActive, Expected: isExpected, Missing: missing, ResolvedPath: resolved, Resolution: resolution, EnvironmentRoot: environmentRoot})
	}
	for _, dir := range strings.Split(getenv("PATH"), sep) {
		if err := ctx.Err(); err != nil {
			return Part{}, err
		}
		dir = strings.Trim(dir, `"`)
		if !filepath.IsAbs(dir) {
			continue
		}
		for _, alias := range aliases {
			p := filepath.Join(dir, alias+suffix)
			info, err := os.Lstat(p)
			packaged := packagedRuntimePath(p, goos)
			if err != nil && !errors.Is(err, os.ErrNotExist) && !packaged {
				inspectionErr = err
			}
			if err != nil || info.IsDir() {
				continue
			}
			resolvedInfo, statErr := os.Stat(p)
			if statErr != nil && !errors.Is(statErr, os.ErrNotExist) && !packaged {
				inspectionErr = statErr
			}
			usable := statErr == nil && resolvedInfo.Mode().IsRegular()
			if goos != "windows" && usable && resolvedInfo.Mode()&0111 == 0 {
				continue
			}
			name := alias
			if name == "python3" {
				name = "python"
			}
			add(p, name, !active[alias] && usable, p == expected[name])
			if usable {
				active[alias] = true
			}
		}
	}
	for _, name := range []string{"python", "node"} {
		if p := expected[name]; p != "" {
			add(p, name, false, true)
		}
	}
	part := Part{Runtimes: out, PythonEnvironment: environment}
	if inspectionErr != nil {
		part.Errors = []CollectorError{{Collector: "runtimes", Message: "runtime metadata inspection failed or was denied"}}
	}
	return part, nil
}

// App execution aliases and protected MSIX files cannot establish interpreter
// resolution through ordinary filesystem APIs. Preserve their unknown state.
func packagedRuntimePath(path, goos string) bool {
	return goos == "windows" && strings.Contains(strings.ToLower(filepath.ToSlash(path)), "/windowsapps/")
}

func knownVenvMetadata(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && (strings.TrimSpace(key) == "home" && strings.TrimSpace(value) != "" || strings.TrimSpace(key) == "version" && numericVersion.MatchString(strings.TrimSpace(value))) {
			return true
		}
	}
	return false
}

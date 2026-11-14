package system

import (
	"bytes"
	"context"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func ReadSmallFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if len(b) > 1<<20 {
		return nil, errors.New("metadata file too large")
	}
	return b, err
}

// FileVersion reads a PE fixed version resource without loading or executing it.
// Unix packages without a version in their installation path remain unknown.
func FileVersion(path string) string {
	f, err := pe.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	section := f.Section(".rsrc")
	if section == nil || section.Size > 4<<20 {
		return ""
	}
	data, err := section.Data()
	if err != nil {
		return ""
	}
	signature := []byte{0xbd, 0x04, 0xef, 0xfe}
	offset := bytes.Index(data, signature)
	if offset < 0 || offset+16 > len(data) {
		return ""
	}
	ms := binary.LittleEndian.Uint32(data[offset+8:])
	ls := binary.LittleEndian.Uint32(data[offset+12:])
	return fmt.Sprintf("%d.%d.%d", ms>>16, ms&65535, ls>>16)
}

// InstalledTool uses fixed installation paths, never the caller's PATH. All
// invocation arguments are fixed by collectors, with no shell interpolation.
func InstalledTool(name string) string {
	var paths []string
	if runtime.GOOS == "windows" {
		root := os.Getenv("SystemRoot")
		programs := os.Getenv("ProgramFiles")
		switch name {
		case "git":
			paths = []string{filepath.Join(programs, "Git", "cmd", "git.exe")}
		case "docker":
			local := os.Getenv("LOCALAPPDATA")
			paths = []string{
				filepath.Join(programs, "Docker", "Docker", "resources", "bin", "docker.exe"),
				filepath.Join(local, "Programs", "DockerDesktop", "resources", "bin", "docker.exe"),
				filepath.Join(local, "Programs", "Docker", "Docker", "resources", "bin", "docker.exe"),
			}
		case "netstat", "tasklist":
			paths = []string{filepath.Join(root, "System32", name+".exe")}
		}
	} else {
		switch name {
		case "lsof":
			paths = []string{"/usr/sbin/lsof", "/usr/bin/lsof"}
		case "docker":
			paths = []string{"/usr/bin/docker", "/usr/local/bin/docker", "/opt/homebrew/bin/docker", "/Applications/Docker.app/Contents/Resources/bin/docker"}
		default:
			paths = []string{"/usr/bin/" + name, "/usr/local/bin/" + name, "/opt/homebrew/bin/" + name}
		}
	}
	for _, p := range paths {
		if filepath.IsAbs(p) {
			if s, err := os.Stat(p); err == nil && s.Mode().IsRegular() {
				return p
			}
		}
	}
	return ""
}

type limitedOutput struct {
	bytes.Buffer
	overflow bool
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len()+n > 1<<20 {
		b.overflow = true
		return 0, errors.New("inspection output exceeds 1 MiB")
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}
func RunTool(ctx context.Context, path string, args ...string) ([]byte, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("tool is not installed in a known location")
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.WaitDelay = 100 * time.Millisecond
	// PATH is not used to resolve the executable. Disallow git work-tree overrides
	// and injected git config while preserving platform essentials.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if strings.HasPrefix(key, "GIT_") || strings.HasPrefix(key, "DOCKER_") {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var stdout limitedOutput
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if stdout.overflow {
		return nil, errors.New("inspection output exceeds 1 MiB")
	}
	return stdout.Bytes(), err
}

// ExitedWith recognizes process exit statuses through wrapped errors.
func ExitedWith(err error, code int) bool {
	var exit interface{ ExitCode() int }
	return errors.As(err, &exit) && exit.ExitCode() == code
}

// Package system is the only package in EnvDoctor allowed to talk to the
// operating system. Collectors call these helpers; rules never do.
//
// Every function here is read-only. Nothing modifies PATH, the registry, shell
// profiles or any file.
package system

import (
	"os"
	"path/filepath"
	"runtime"
)

// HomeDir returns the current user's home directory.
func HomeDir() (string, error) {
	return os.UserHomeDir()
}

// Getenv reads an environment variable. It is a thin wrapper so that collectors
// can be given a fake environment in tests.
func Getenv(key string) string {
	return os.Getenv(key)
}

// Stat returns file metadata. Collectors use it to check PATH entries without
// executing anything from the PATH.
func Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// PlatformInfo returns a human-readable operating system version and whether
// the current process is elevated. It never fails: if the version cannot be
// determined the string is empty, because a missing version is a cosmetic gap
// and not a reason to abort a diagnosis.
func PlatformInfo() (version string, elevated bool) {
	return platformVersion(), isElevated()
}

// ShellInfo returns the name and path of the shell that started the process.
// Path is empty when only the shell family is known, which happens on Windows
// when PowerShell sets PSModulePath but not SHELL.
func ShellInfo() (name, path string) {
	if shell := os.Getenv("SHELL"); shell != "" {
		return filepath.Base(shell), shell
	}
	if runtime.GOOS == "windows" {
		if os.Getenv("PSModulePath") != "" {
			return "powershell", ""
		}
		if comspec := os.Getenv("ComSpec"); comspec != "" {
			return "cmd", comspec
		}
	}
	return "unknown", ""
}

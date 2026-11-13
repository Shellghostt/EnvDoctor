//go:build linux

package system

import (
	"os"
	"strings"
)

// platformVersion reads PRETTY_NAME from os-release. If that file is missing
// (minimal containers, some distributions) the kernel release is not a useful
// substitute for a distribution name, so an empty string is returned.
func platformVersion() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || key != "PRETTY_NAME" {
			continue
		}
		return strings.Trim(value, `"'`)
	}
	return ""
}

func isElevated() bool {
	return os.Geteuid() == 0
}

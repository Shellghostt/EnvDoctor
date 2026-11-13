//go:build darwin

package system

import (
	"os"

	"golang.org/x/sys/unix"
)

// platformVersion asks the kernel for the product version. sysctl
// kern.osproductversion is available on every supported macOS release, so no
// external binary is executed.
func platformVersion() string {
	if version, err := unix.Sysctl("kern.osproductversion"); err == nil && version != "" {
		return version
	}
	if release, err := unix.Sysctl("kern.osrelease"); err == nil && release != "" {
		return "Darwin kernel " + release
	}
	return ""
}

func isElevated() bool {
	return os.Geteuid() == 0
}

//go:build windows

package system

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// platformVersion maps the NT build number to a marketing name plus the build
// number. The build number is the only part that is exact; Windows 11 23H2
// cannot be derived from the major version alone, so it is not claimed.
func platformVersion() string {
	info := windows.RtlGetVersion()
	if info == nil {
		return ""
	}

	name := "10"
	if info.BuildNumber >= 22000 {
		name = "11"
	}
	return fmt.Sprintf("%s (build %d)", name, info.BuildNumber)
}

// isElevated asks the process token directly instead of trying to open a
// privileged resource, which would be a side effect.
func isElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

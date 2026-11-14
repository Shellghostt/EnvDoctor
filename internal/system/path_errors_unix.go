//go:build linux || darwin

package system

import (
	"errors"
	"syscall"
)

func isMalformedPath(err error) bool {
	return errors.Is(err, syscall.ENOTDIR) || errors.Is(err, syscall.ENAMETOOLONG)
}

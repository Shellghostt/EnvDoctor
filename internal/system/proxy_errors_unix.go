//go:build !windows

package system

import (
	"errors"
	"syscall"
)

func proxyUnreachableError(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENETUNREACH) || errors.Is(err, syscall.EHOSTUNREACH)
}

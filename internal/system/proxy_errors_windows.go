//go:build windows

package system

import (
	"errors"
	"golang.org/x/sys/windows"
)

func proxyUnreachableError(err error) bool {
	return errors.Is(err, windows.WSAECONNREFUSED) || errors.Is(err, windows.WSAENETUNREACH) || errors.Is(err, windows.WSAEHOSTUNREACH)
}

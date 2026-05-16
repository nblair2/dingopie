//go:build windows

package shell

import (
	"errors"
	"io"
)

var errUnsupported = errors.New("shell mode is not supported on Windows")

func ClientShell(_ io.Writer, ip string, port int, key, command string) error {
	return errUnsupported
}

func ServerShell(_ io.Writer, ip string, port int, key, command string) error {
	return errUnsupported
}

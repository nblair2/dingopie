//go:build windows

package shell

import "errors"

var errUnsupported = errors.New("shell mode is not supported on Windows")

func ClientShell(ip string, port int, key, command string) error {
	return errUnsupported
}

func ServerShell(ip string, port int, key, command string) error {
	return errUnsupported
}

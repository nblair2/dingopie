//go:build windows

package inject

import (
	"errors"
	"io"
)

var errUnsupported = errors.New("inject mode is not supported on Windows")

func ClientInjectReceive(
	_ io.Writer,
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
) ([]byte, error) {
	return nil, errUnsupported
}

func ServerInjectSend(
	_ io.Writer,
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
	data []byte,
) error {
	return errUnsupported
}

func ClientInjectSend(
	_ io.Writer,
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
	data []byte,
) error {
	return errUnsupported
}

func ServerInjectReceive(
	_ io.Writer,
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
) ([]byte, error) {
	return nil, errUnsupported
}

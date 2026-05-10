//go:build windows

package inject

import "errors"

var errUnsupported = errors.New("inject mode is not supported on Windows")

func ClientInjectReceive(
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
) ([]byte, error) {
	return nil, errUnsupported
}

func ServerInjectSend(
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
	data []byte,
) error {
	return errUnsupported
}

func ClientInjectSend(
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
	data []byte,
) error {
	return errUnsupported
}

func ServerInjectReceive(
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
) ([]byte, error) {
	return nil, errUnsupported
}

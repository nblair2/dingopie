//go:build !windows

package shell

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/creack/pty"
)

// shell initiates an interactive shell session over the provided stream.
func shell(out io.Writer, command string, stream dnp3Stream, maxDataLen int) error {
	var c *exec.Cmd

	if strings.HasSuffix(command, "bash") {
		rcContent := `PS1="dingopie> "`
		//nolint:gosec //G204 user provided command which they must have permissions to run
		c = exec.CommandContext(
			context.Background(),
			"bash",
			"-c",
			fmt.Sprintf("exec %s --rcfile <(echo '%s') -i", command, rcContent),
		)
	} else {
		c = exec.CommandContext(context.Background(), command)
	}

	ptmx, err := pty.Start(c)
	if err != nil {
		return fmt.Errorf("error starting pty: %w", err)
	}
	defer ptmx.Close()

	buf := make([]byte, maxDataLen)

	go func() { _, _ = io.Copy(ptmx, stream) }()

	_, _ = io.CopyBuffer(stream, ptmx, buf)

	fmt.Fprintf(out, ">> Shell session ended\n")

	return nil
}

// ClientShell - dingopie client direct shell.
func ClientShell(out io.Writer, ip string, port int, key, command string) error {
	//nolint:exhaustruct_v5 // zero-value Dialer, just need DialContext for noctx
	conn, err := (&net.Dialer{}).DialContext(
		context.Background(),
		"tcp",
		net.JoinHostPort(ip, strconv.Itoa(port)),
	)
	if err != nil {
		return fmt.Errorf("error connecting: %w", err)
	}
	defer conn.Close()

	fmt.Fprintf(out, ">> Connected to %s:%d\n", ip, port)

	stream := newClientStream(key, conn)

	return shell(out, command, stream, clientMaxDataLen)
}

// ServerShell - dingopie server direct shell.
func ServerShell(out io.Writer, ip string, port int, key, command string) error {
	//nolint:exhaustruct_v5 // zero-value ListenConfig, just need Listen for noctx
	ln, err := (&net.ListenConfig{}).Listen(
		context.Background(),
		"tcp",
		net.JoinHostPort(ip, strconv.Itoa(port)),
	)
	if err != nil {
		return fmt.Errorf("error starting TCP listener: %w", err)
	}
	defer ln.Close()

	fmt.Fprintf(out, ">> Listening on %s:%d\n", ip, port)

	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("error accepting connection: %w", err)
	}
	defer conn.Close()

	fmt.Fprintf(out, "\tConnection %s\n", conn.RemoteAddr().String())
	stream := newServerStream(key, conn)

	return shell(out, command, stream, serverMaxDataLen)
}

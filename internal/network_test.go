package internal_test

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/nblair2/dingopie/internal"
)

func TestSendMessage_HeaderDataMismatch(t *testing.T) {
	t.Parallel()

	sendChan := make(chan []byte, 1)

	err := internal.SendMessage(
		nil,
		[][]byte{internal.DNP3ReadClass1, internal.DNP3ReadClass2},
		[][]byte{nil},
		sendChan,
	)
	if !errors.Is(err, internal.ErrHeaderDataMismatch) {
		t.Errorf("errors.Is(err, internal.ErrHeaderDataMismatch) = false, err: %v", err)
	}
}

func TestReceiveAndValidate_UnexpectedHeaderCount(t *testing.T) {
	t.Parallel()

	recvChan := make(chan []byte, 1)
	recvChan <- makeFrame(t, internal.DNP3ReadClass1, nil)

	_, err := internal.ReceiveAndValidate(
		recvChan,
		[][]byte{internal.DNP3ReadClass1, internal.DNP3ReadClass2},
	)
	if !errors.Is(err, internal.ErrUnexpectedHeaderCount) {
		t.Errorf("errors.Is(err, internal.ErrUnexpectedHeaderCount) = false, err: %v", err)
	}
}

func TestReceiveAndValidate_UnexpectedSignal(t *testing.T) {
	t.Parallel()

	recvChan := make(chan []byte, 1)
	recvChan <- makeFrame(t, internal.DNP3ReadClass1, nil)

	_, err := internal.ReceiveAndValidate(recvChan, [][]byte{internal.DNP3ReadClass2})
	if !errors.Is(err, internal.ErrUnexpectedSignal) {
		t.Errorf("errors.Is(err, internal.ErrUnexpectedSignal) = false, err: %v", err)
	}
}

// TestClientHandleConn_ConnectionClosed verifies that when the remote end of the
// connection closes, ClientHandleConn returns an error matchable both as
// internal.ErrConnectionClosed and (via wrapping) io.EOF.
func TestClientHandleConn_ConnectionClosed(t *testing.T) {
	t.Parallel()

	client, remote := net.Pipe()

	write := make(chan []byte, 1)
	read := make(chan []byte, 1)

	write <- []byte{0x01}

	done := make(chan error, 1)

	go func() {
		done <- internal.ClientHandleConn(client, write, read)
	}()

	// Drain the write, then close our end so the next client.Read sees EOF.
	buf := make([]byte, 16)

	_, err := remote.Read(buf)
	if err != nil {
		t.Fatalf("remote.Read: %v", err)
	}

	err = remote.Close()
	if err != nil {
		t.Fatalf("remote.Close: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, internal.ErrConnectionClosed) {
			t.Errorf("errors.Is(err, internal.ErrConnectionClosed) = false, err: %v", err)
		}

		if !errors.Is(err, io.EOF) {
			t.Errorf("errors.Is(err, io.EOF) = false, err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ClientHandleConn to return")
	}
}

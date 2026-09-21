package shell

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/nblair2/dingopie/internal"
)

func TestDNP3Stream_Write_DataTooLarge(t *testing.T) {
	t.Parallel()

	// maxDataLen intentionally not a multiple of dnp3PointSize, so padding overflows it.
	ds := dnp3Stream{
		primary:    true,
		frame:      internal.NewDNP3RequestFrame(),
		conn:       nil,
		txCipher:   internal.NewCipherStream("k"),
		rxCipher:   internal.NewCipherStream("k"),
		txSendSize: reqSendSize,
		txSendData: reqSendData,
		rxSendSize: respSendSize,
		rxSendData: respSendData,
		maxDataLen: 3,
	}

	_, err := ds.Write([]byte{0x01, 0x02, 0x03})
	if !errors.Is(err, ErrDataTooLarge) {
		t.Errorf("errors.Is(err, ErrDataTooLarge) = false, err: %v", err)
	}
}

func TestDNP3Stream_Read_BufferTooSmall(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()

	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	client := newClientStream("test-key", clientConn)
	server := newServerStream("test-key", serverConn)

	writeErr := make(chan error, 1)

	go func() {
		_, err := server.Write([]byte("hello world"))
		writeErr <- err
	}()

	buf := make([]byte, 1)

	readDone := make(chan struct {
		err error
	}, 1)

	go func() {
		_, err := client.Read(buf)
		readDone <- struct{ err error }{err}
	}()

	select {
	case res := <-readDone:
		if !errors.Is(res.err, ErrBufferTooSmall) {
			t.Errorf("errors.Is(err, ErrBufferTooSmall) = false, err: %v", res.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for client.Read")
	}

	select {
	case err := <-writeErr:
		if err != nil {
			t.Fatalf("server.Write: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server.Write")
	}
}

func TestDNP3Stream_ProcessFrame_InvalidResponse(t *testing.T) {
	t.Parallel()

	clientConn, otherEnd := net.Pipe()

	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = otherEnd.Close()
	})

	client := newClientStream("test-key", clientConn)

	// Craft a frame using the *request* headers (reqSendSize/reqSendData), which do not
	// match what the client stream expects to receive (respSendSize/respSendData).
	badFrame := internal.NewDNP3ResponseFrame()

	msg, err := internal.MakeDNP3Bytes(
		&badFrame,
		reqSendSize, []byte{0x00, 0x00, 0x00},
		reqSendData, []byte{0x00, 0x00, 0x00, 0x00, 0x00},
	)
	if err != nil {
		t.Fatalf("MakeDNP3Bytes: %v", err)
	}

	writeDone := make(chan error, 1)

	go func() {
		_, werr := otherEnd.Write(msg)
		writeDone <- werr
	}()

	buf := make([]byte, 128)

	readDone := make(chan struct {
		err error
	}, 1)

	go func() {
		_, rerr := client.Read(buf)
		readDone <- struct{ err error }{rerr}
	}()

	select {
	case res := <-readDone:
		if !errors.Is(res.err, ErrInvalidResponse) {
			t.Errorf("errors.Is(err, ErrInvalidResponse) = false, err: %v", res.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for client.Read")
	}

	select {
	case werr := <-writeDone:
		if werr != nil {
			t.Fatalf("otherEnd.Write: %v", werr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for otherEnd.Write")
	}
}

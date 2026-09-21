package primary

import (
	"errors"
	"testing"
	"time"

	"github.com/nblair2/dingopie/internal"
)

// TestClientExchangeAck_EchoMismatch verifies that clientExchangeAck returns an error
// matchable as ErrEchoMismatch when the outstation echoes back different data than was sent.
//
//nolint:paralleltest // exercises shared package-level frame/sendChan/recvChan state
func TestClientExchangeAck_EchoMismatch(t *testing.T) {
	frame = internal.NewDNP3RequestFrame()

	sent := [][]byte{{0x01, 0x02, 0x03, 0x04, 0x00}}
	echoed := [][]byte{{0xFF, 0xFF, 0xFF, 0xFF, 0x00}}

	done := make(chan struct{})

	go func() {
		defer close(done)

		<-sendChan // drain the sent message

		respFrame := internal.NewDNP3ResponseFrame()

		respBytes, err := internal.MakeDNP3Bytes(&respFrame, internal.DNP3G41V1Q0, echoed[0])
		if err != nil {
			t.Errorf("MakeDNP3Bytes: %v", err)

			return
		}

		recvChan <- respBytes
	}()

	errChan := make(chan error, 1)

	go func() {
		errChan <- clientExchangeAck([][]byte{internal.DNP3G41V1Q0}, sent)
	}()

	var err error

	select {
	case err = <-errChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for clientExchangeAck to return")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for echo goroutine to finish")
	}

	if !errors.Is(err, ErrEchoMismatch) {
		t.Errorf("errors.Is(err, ErrEchoMismatch) = false, err: %v", err)
	}
}

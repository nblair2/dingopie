package secondary

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/nblair2/dingopie/internal"
)

// TestClientReceiveProcess_UnexpectedSizeLength verifies that clientReceiveProcess returns
// an error matchable as ErrUnexpectedSizeLength when the server's size handshake response
// does not decode to exactly sizePrefixBytes bytes.
//
//nolint:paralleltest // exercises shared package-level frame/sendChan/recvChan state
func TestClientReceiveProcess_UnexpectedSizeLength(t *testing.T) {
	frame = internal.NewDNP3RequestFrame()

	done := make(chan struct{})

	go func() {
		defer close(done)

		<-sendChan // drain the initiate-connection request

		respFrame := internal.NewDNP3ResponseFrame()

		// DNP3G30V4Q0 has a 2-byte point size; one point (2 bytes) is not sizePrefixBytes (4).
		respBytes, err := internal.MakeDNP3Bytes(
			&respFrame,
			internal.DNP3G30V4Q0,
			[]byte{0xAA, 0xBB},
		)
		if err != nil {
			t.Errorf("MakeDNP3Bytes: %v", err)

			return
		}

		recvChan <- respBytes
	}()

	resultChan := make(chan recvResult, 1)

	go func() {
		resultChan <- clientReceiveProcess(io.Discard, 0)
	}()

	var result recvResult

	select {
	case result = <-resultChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for clientReceiveProcess to return")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for echo goroutine to finish")
	}

	if !errors.Is(result.err, ErrUnexpectedSizeLength) {
		t.Errorf("errors.Is(err, ErrUnexpectedSizeLength) = false, err: %v", result.err)
	}
}

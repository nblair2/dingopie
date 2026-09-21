package internal

import (
	"errors"
	"fmt"
	"io"
	"net"
	"slices"

	"github.com/nblair2/go-dnp3/v4/dnp3"
)

// TCPReadBufferSize is the buffer length used when reading from a TCP connection.
const TCPReadBufferSize = 4096

var (
	// ErrConnectionClosed indicates the remote host closed the connection (wraps io.EOF).
	ErrConnectionClosed = fmt.Errorf("connection closed by remote host: %w", io.EOF)
	// ErrHeaderDataMismatch indicates a mismatched number of DNP3 object headers and data
	// slices were provided/received.
	ErrHeaderDataMismatch = errors.New("headers and data length mismatch")
	// ErrUnexpectedHeaderCount indicates a received DNP3 message did not contain the
	// expected number of object headers.
	ErrUnexpectedHeaderCount = errors.New("unexpected number of expected headers")
	// ErrUnexpectedSignal indicates a received DNP3 object header did not match the header
	// expected at that position.
	ErrUnexpectedSignal = errors.New("unexpected signal received")
)

// ClientHandleConn manages a client (DNP3 master) connection, in pairs of write/read.
func ClientHandleConn(conn net.Conn, write <-chan []byte, read chan<- []byte) error {
	for {
		_, err := conn.Write(<-write)
		if err != nil {
			return fmt.Errorf("error writing to connection: %w", err)
		}

		buf := make([]byte, TCPReadBufferSize)

		n, err := conn.Read(buf)
		if errors.Is(err, io.EOF) {
			return ErrConnectionClosed
		} else if err != nil {
			return fmt.Errorf("error reading from connection: %w", err)
		}

		msg := make([]byte, n)
		copy(msg, buf[:n])

		read <- msg
	}
}

// ServerHandleConn manages a server (DNP3 outstation) connection, in pairs of read/write.
func ServerHandleConn(conn net.Conn, read chan<- []byte, write <-chan []byte) error {
	for {
		buf := make([]byte, TCPReadBufferSize)

		n, err := conn.Read(buf)
		if errors.Is(err, io.EOF) {
			return nil // success
		} else if err != nil {
			return fmt.Errorf("error reading from connection: %w", err)
		}

		msg := make([]byte, n)
		copy(msg, buf[:n])

		read <- msg

		resp := <-write

		_, err = conn.Write(resp)
		if err != nil {
			return fmt.Errorf("error writing to connection: %w", err)
		}
	}
}

// SendMessage constructs a DNP3 message from the pairs of headers and data, and sends it on the sendChan.
func SendMessage(frame *dnp3.Frame, headers, data [][]byte, sendChan chan<- []byte) error {
	if data == nil {
		data = make([][]byte, len(headers))
	} else if len(headers) != len(data) {
		return ErrHeaderDataMismatch
	}

	sendPairs := make([][]byte, 0, len(headers)+len(data))
	for i := range headers {
		sendPairs = append(sendPairs, headers[i], data[i])
	}

	msg, err := MakeDNP3Bytes(frame, sendPairs...)
	if err != nil {
		return fmt.Errorf("error making DNP3 bytes: %w", err)
	}

	sendChan <- msg

	return nil
}

// ReceiveAndValidate waits for a message on the channel, parses it, and validates the headers.
func ReceiveAndValidate(recvChan <-chan []byte, expectedHeaders [][]byte) ([][]byte, error) {
	msg := <-recvChan

	headers, data, err := GetObjectDataFromDNP3Bytes(msg)
	switch {
	case err != nil:
		return nil, fmt.Errorf("error getting signal from DNP3 bytes: %w", err)
	case len(headers) != len(data):
		return nil, fmt.Errorf(
			"%w: %d headers, %d data",
			ErrHeaderDataMismatch,
			len(headers),
			len(data),
		)
	case len(expectedHeaders) != len(headers):
		return nil, fmt.Errorf(
			"%w: expected %d, received %d",
			ErrUnexpectedHeaderCount,
			len(expectedHeaders),
			len(headers),
		)
	}

	for i, expHdr := range expectedHeaders {
		if !slices.Equal(expHdr, headers[i]) {
			return nil, fmt.Errorf(
				"%w %v, expected %v",
				ErrUnexpectedSignal,
				headers[i],
				expHdr,
			)
		}
	}

	return data, nil
}

// ClientExchange handles a single send/receive cycle, sending a message and waiting for a response.
// Message is constructed from sendHeader and sendData, pairs of byte slices representing DNP3 object headers and
// associated data. If sendData is nil, empty data slices are used (eg: for ReadClassX requests). Responses are
// validated against recvHeader (also a slice of byte slices representing expected DNP3 object headers). Data in the
// response is returned as a slice of byte slices, with each index corresponding to the data for each header in
// recvHeader.
func ClientExchange(
	frame *dnp3.Frame,
	sendHeader, recvHeader, sendData [][]byte,
	sendChan chan<- []byte,
	recvChan <-chan []byte,
) ([][]byte, error) {
	err := SendMessage(frame, sendHeader, sendData, sendChan)
	if err != nil {
		return nil, err
	}

	return ReceiveAndValidate(recvChan, recvHeader)
}

// ServerExchange handles a single receive/send cycle, waiting for a message and sending a response.
// The received message is validated against recvHeader (a slice of byte slices representing expected DNP3 object
// headers). Data in the received message is returned as a slice of byte slices, with each index corresponding to the
// data for the corresponding header in recvHeader. Once the message is validated, a response is sent. The response is
// constructed from sendHeader and sendData, pairs of byte slices representing DNP3 object headers and
// associated data. If sendData is nil, empty data slices are used.
func ServerExchange(
	frame *dnp3.Frame,
	recvHeaders, sendHeader, sendData [][]byte,
	recvChan <-chan []byte,
	sendChan chan<- []byte,
) ([][]byte, error) {
	data, err := ReceiveAndValidate(recvChan, recvHeaders)
	if err != nil {
		return nil, err
	}

	err = SendMessage(frame, sendHeader, sendData, sendChan)
	if err != nil {
		return nil, err
	}

	return data, nil
}

package internal

import (
	"errors"
	"fmt"
	"io"
	"net"
	"slices"

	"github.com/nblair2/go-dnp3/dnp3"
)

// ClientHandleConn manages a client (DNP3 master) connection, in pairs of write/read.
func ClientHandleConn(conn net.Conn, write <-chan []byte, read chan<- []byte) error {
	for {
		_, err := conn.Write(<-write)
		if err != nil {
			return fmt.Errorf("error writing to connection: %w", err)
		}

		buf := make([]byte, 4096)

		n, err := conn.Read(buf)
		if errors.Is(err, io.EOF) {
			return errors.New("connection closed by remote host")
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
		buf := make([]byte, 4096)

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
		return errors.New("headers and data length mismatch")
	}

	sendPairs := make([][]byte, 0, len(headers)*2)
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
			"received headers and data lengths do not match: %d headers, %d data",
			len(headers),
			len(data),
		)
	case len(expectedHeaders) != len(headers):
		return nil, fmt.Errorf(
			"unexpected number of expected headers: %d, received %d",
			len(expectedHeaders),
			len(headers),
		)
	}

	for i, expHdr := range expectedHeaders {
		if !slices.Equal(expHdr, headers[i]) {
			return nil, fmt.Errorf(
				"unexpected signal received %v, expected %v",
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

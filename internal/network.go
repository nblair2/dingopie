package internal

import (
	"errors"
	"fmt"
	"io"
	"net"
	"slices"

	"github.com/nblair2/go-dnp3/dnp3"
)

// ClientHandleConn handles a client connection, writing to a connection and reading responses.
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

// ServerHandleConn handles a server connection, reading from a connection and writing responses.
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

// ClientExchange handles a single send/receive cycle.
func ClientExchange(
	frame dnp3.Frame,
	sendHeader, recvHeader, sendData []byte,
	sendChan chan<- []byte,
	recvChan <-chan []byte,
) ([]byte, error) {
	msg, err := MakeDNP3Bytes(frame, sendHeader, sendData)
	if err != nil {
		return nil, fmt.Errorf("error making DNP3 bytes: %w", err)
	}

	sendChan <- msg

	msg = <-recvChan

	header, recvData, err := GetObjectDataFromDNP3Bytes(msg)
	if err != nil {
		return nil, fmt.Errorf("error getting signal from DNP3 bytes: %w", err)
	} else if !slices.Equal(recvHeader, header) {
		return nil, fmt.Errorf("unexpected signal received %v, expected %v", header, recvHeader)
	}

	return recvData, nil
}

// ServerExchange handles a single receive/send cycle.
func ServerExchange(
	frame dnp3.Frame,
	recvHeader, sendHeader, sendData []byte,
	recvChan <-chan []byte,
	sendChan chan<- []byte,
) ([]byte, error) {
	msg := <-recvChan

	header, recvData, err := GetObjectDataFromDNP3Bytes(msg)
	if err != nil {
		return nil, fmt.Errorf("error getting signal from bytes: %w", err)
	} else if !slices.Equal(recvHeader, header) {
		return nil, fmt.Errorf("unexpected signal received %v, expected %v", header, recvHeader)
	}

	msg, err = MakeDNP3Bytes(frame, sendHeader, sendData)
	if err != nil {
		return nil, fmt.Errorf("error making dnp3 bytes: %w", err)
	}

	sendChan <- msg

	return recvData, nil
}

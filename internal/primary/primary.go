// Package primary sends data from client (master) --> server (outstation). The client initiates the connection, the
// server acks with random data, then the client sends the size (in bytes) of the data to be sent (before padding).
// The size allows the server to strip any padding after the transfer is complete. After this 'handshake' the client
// periodically sends 'Send Data' requests to the server. The interval between these requests is configurable, with
// the 'wait' flag. The client also determines the size of each data 'chunk' to send with the 'objects' flag. The
// server responds to each requests by echoing the same data back (acknowledging the CROB). Once the client has
// transferred all of its data (and perhaps a little padding), it sends a disconnect message containing some random
// bytes. The server acks the disconnect and the connection is closed.
//
// The sequence of messages is as follows:
//
//		(master)--- ReadClass1230  -->(outstation)  Initiate connection
//		(master)<-- G30V4Q0 + rand ---(outstation)  Ack with random
//		(master)--- G41V2Q0 + size ---(outstation)  Send Size
//		(master)<-- G41V2Q0 + rand ---(outstation)  Ack with random
//	Loop:
//		(master)--- G41V1Q0 + data -->(outstation)  Send Data
//		(master)<-- G41V1Q0 + data ---(outstation)  Ack Data
//		     ...
//	 End:
//		(master)--- ReadClass123   -->(outstation)  Disconnect
//		(master)<-- G30V3Q0 + rand ---(outstation)  AckDisconnect
package primary

import (
	"bytes"
	"dingopie/internal"
	"encoding/binary"
	"fmt"
	"net"
	"slices"
	"strconv"
	"time"

	"github.com/nblair2/go-dnp3/dnp3"
)

// ==================================================================
// COMMON
// ==================================================================

var (
	initiateConnection = internal.DNP3ReadClass1230
	ackConnect         = [][]byte{internal.DNP3G30V4Q0}
	sendSize           = [][]byte{internal.DNP3G41V2Q0}
	ackSize            = [][]byte{internal.DNP3G41V2Q0}
	sendData           = [][]byte{internal.DNP3G41V1Q0}
	ackData            = [][]byte{internal.DNP3G41V1Q0}
	disconnect         = internal.DNP3ReadClass123
	ackDisconnect      = [][]byte{internal.DNP3G30V3Q0}
	sendChan           = make(chan []byte)
	recvChan           = make(chan []byte)
	frame              dnp3.Frame
	dataSeq            internal.DataSequence
)

// ==================================================================
// CLIENT SEND
// ==================================================================

type recvResult struct {
	data []byte
	err  error
}

// ClientSend - dingopie client direct send.
func ClientSend(ip string, port int, data []byte, wait time.Duration, objects int) error {
	var err error

	dataSeq, err = internal.NewDataSequence(data, objects)
	if err != nil {
		return fmt.Errorf("error creating data sequence: %w", err)
	}

	frame = internal.NewDNP3RequestFrame()

	conn, err := net.Dial("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("error connecting: %w", err)
	}
	defer conn.Close()

	fmt.Printf(">> Connected to %s:%d\n", ip, port)

	connErrChan := make(chan error, 1)
	procErrChan := make(chan error, 1)

	go func() { connErrChan <- internal.ClientHandleConn(conn, sendChan, recvChan) }()
	go func() { procErrChan <- clientSendProcess(wait) }()

	for {
		select {
		case err := <-connErrChan:
			return fmt.Errorf("error with connection: %w", err)
		case err := <-procErrChan:
			return err
		}
	}
}

func clientSendProcess(wait time.Duration) error {
	_, err := internal.ClientExchange(
		&frame,
		initiateConnection,
		ackConnect,
		nil,
		sendChan,
		recvChan,
	)
	if err != nil {
		return fmt.Errorf("error during connect exchange: %w", err)
	}

	time.Sleep(wait)
	// Set function code to Direct Operate for sending data client -> server
	frame.Application.SetFunctionCode(byte(dnp3.DirOperate))

	sizeBytes, err := internal.InsertPeriodicBytes(dataSeq.SizeBytes, []byte{0x00}, 2, 2)
	if err != nil {
		return fmt.Errorf("error preparing size bytes: %w", err)
	}

	_, err = internal.ClientExchange(
		&frame,
		sendSize,
		ackSize,
		[][]byte{sizeBytes},
		sendChan,
		recvChan,
	)
	if err != nil {
		return fmt.Errorf("error during send size exchange: %w", err)
	}

	bar := internal.NewProgressBar(dataSeq.OriginalLength, ">>>> Sending: ")
	for _, chunk := range dataSeq.DataChunks {
		time.Sleep(wait)

		data, err := internal.InsertPeriodicBytes(chunk, []byte{0x00}, 4, 4)
		if err != nil {
			return fmt.Errorf("error preparing data chunk: %w", err)
		}

		err = clientExchangeAck(sendData, [][]byte{data})
		if err != nil {
			return fmt.Errorf("error during send data exchange: %w", err)
		}

		bar.Add(len(chunk))
	}

	bar.Finish()
	time.Sleep(wait)
	// Set function code back to Read for disconnect
	frame.Application.SetFunctionCode(byte(dnp3.Read))

	_, err = internal.ClientExchange(&frame, disconnect, ackDisconnect, nil, sendChan, recvChan)
	if err != nil {
		return fmt.Errorf("error during disconnect exchange: %w", err)
	}

	return nil
}

func clientExchangeAck(headers, data [][]byte) error {
	if len(headers) != len(data) {
		return fmt.Errorf(
			"header and data length mismatch: %d headers, %d data",
			len(headers),
			len(data),
		)
	}

	sendPairs := make([][]byte, 0, len(headers)*2)
	for i := range headers {
		sendPairs = append(sendPairs, headers[i], data[i])
	}

	msg, err := internal.MakeDNP3Bytes(&frame, sendPairs...)
	if err != nil {
		return fmt.Errorf("error making DNP3 bytes: %w", err)
	}

	sendChan <- msg

	msg = <-recvChan

	recvHeaders, recvData, err := internal.GetObjectDataFromDNP3Bytes(msg)
	switch {
	case err != nil:
		return fmt.Errorf("error getting signal from DNP3 bytes: %w", err)
	case len(recvHeaders) != len(recvData):
		return fmt.Errorf(
			"send headers and data lengths do not match: %d headers, %d data",
			len(recvHeaders),
			len(recvData),
		)
	case len(recvHeaders) != len(headers):
		return fmt.Errorf(
			"unexpected number of expected headers: %d, received %d",
			len(headers),
			len(recvHeaders),
		)
	}

	for i, recvHdr := range recvHeaders {
		if !slices.Equal(recvHdr, headers[i]) {
			return fmt.Errorf("unexpected signal received %v, expected %v", recvHdr, headers[i])
		} else if !slices.Equal(recvData[i], data[i]) {
			return fmt.Errorf("unexpected data received %v, expected %v", recvData[i], data[i])
		}
	}

	return nil
}

// ==================================================================
// SERVER RECEIVE
// ==================================================================

// ServerReceive - dingopie server direct receive.
func ServerReceive(ip string, port int) ([]byte, error) {
	frame = internal.NewDNP3ResponseFrame()

	// Open socket, wait for connection
	socket := fmt.Sprintf("%s:%d", ip, port)

	ln, err := net.Listen("tcp", socket)
	if err != nil {
		return nil, fmt.Errorf("error starting TCP listener: %w", err)
	}

	defer ln.Close()

	fmt.Printf(">> Listening on %s\n", socket)

	conn, err := ln.Accept()
	fmt.Printf(">> Client connected: %s\n", conn.RemoteAddr().String())

	if err != nil {
		return nil, fmt.Errorf("error accepting connection: %w", err)
	}

	defer conn.Close()

	// run go funcs
	connErrChan := make(chan error, 1)
	procErrChan := make(chan recvResult, 1)

	go func() { connErrChan <- internal.ServerHandleConn(conn, recvChan, sendChan) }()
	go func() { procErrChan <- serverReceiveProcess() }()

	var result recvResult

	for {
		select {
		case err := <-connErrChan:
			if err != nil {
				return result.data, fmt.Errorf("error with connection: %w", err)
			}

			return result.data, nil
		case result = <-procErrChan:
			if result.err != nil {
				return result.data, result.err
			}
			// // if no errV
			// <-connErrChan
			// return result.data, nil
		}
	}
}

func serverReceiveProcess() recvResult {
	var data []byte

	// Initiate connection
	_, err := internal.ServerExchange(
		&frame,
		initiateConnection,
		ackConnect,
		[][]byte{internal.NewRandomBytes(4)},
		recvChan,
		sendChan,
	)
	if err != nil {
		return recvResult{nil, fmt.Errorf("error during handshake: %w", err)}
	}

	// Get data size
	dataSlice, err := serverExchangeAck(sendSize, ackSize)
	if err != nil {
		return recvResult{nil, fmt.Errorf("error during size exchange: %w", err)}
	}

	sizeBytes := bytes.Join(dataSlice, nil)

	sizeBytes, err = internal.RemovePeriodicBytes(sizeBytes, 1, 2, 2)
	if err != nil {
		return recvResult{nil, fmt.Errorf("error processing size bytes: %w", err)}
	}

	size := int(binary.BigEndian.Uint32(sizeBytes))

	// Receive data loop
	bar := internal.NewProgressBar(size, ">>>> Receiving: ")

	for len(data) < size {
		recvDataSlice, err := serverExchangeAck(sendData, ackData)
		if err != nil {
			return recvResult{data, fmt.Errorf("error during data exchange: %w", err)}
		}

		recvData, err := internal.RemovePeriodicBytes(bytes.Join(recvDataSlice, nil), 1, 4, 4)
		if err != nil {
			return recvResult{data, fmt.Errorf("error processing received data: %w", err)}
		}

		data = append(data, recvData...)
		bar.Add(len(recvData))
	}

	// Disconnect
	bar.Finish()

	_, err = internal.ServerExchange(
		&frame,
		disconnect,
		ackDisconnect,
		[][]byte{internal.NewRandomBytes(4)},
		recvChan,
		sendChan,
	)

	return recvResult{data[:size], err}
}

func serverExchangeAck(expectedHeaders, responseHeaders [][]byte) ([][]byte, error) {
	if len(expectedHeaders) != len(responseHeaders) {
		return nil, fmt.Errorf(
			"expected and response header length mismatch: %d expected, %d response",
			len(expectedHeaders),
			len(responseHeaders),
		)
	}

	recvData := <-recvChan

	headers, data, err := internal.GetObjectDataFromDNP3Bytes(recvData)
	switch {
	case err != nil:
		return nil, fmt.Errorf("error getting signal from bytes: %w", err)
	case len(headers) != len(data):
		return nil, fmt.Errorf(
			"received headers and data length mismatch: %d headers, %d data",
			len(headers),
			len(data),
		)
	case len(headers) != len(expectedHeaders):
		return nil, fmt.Errorf(
			"unexpected number of expected headers: %d, received %d",
			len(expectedHeaders),
			len(headers),
		)
	}

	for i, expHdr := range expectedHeaders {
		if !slices.Equal(expHdr, headers[i]) {
			return nil, fmt.Errorf("unexpected signal received %v, expected %v", headers[i], expHdr)
		}
	}

	sendPairs := make([][]byte, 0, len(responseHeaders)*2)
	for i := range responseHeaders {
		sendPairs = append(sendPairs, responseHeaders[i], data[i])
	}

	msg, err := internal.MakeDNP3Bytes(&frame, sendPairs...)
	if err != nil {
		return nil, fmt.Errorf("error making DNP3 bytes: %w", err)
	}

	sendChan <- msg

	return data, nil
}

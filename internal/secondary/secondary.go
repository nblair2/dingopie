// Package secondary is sending data server (outstation) --> client (master). The server waits for client connection.
// When client connects, the server responds with the size(in bytes) of the data to be sent (before padding).
// This size is essential so that the client knows how much of the received data to 'throw out' after the end of
// transfer. After this 'handshake' the client periodically polls the server to 'Get Data'. The interval between these
// polls is configurable, the 'wait' flag. The server responds to each of these requests with a 'chunk' of data.
// The size of these chunks is configurable, the 'objects' flag will determine how many 4-byte objects to send in each
// response. Once the server has sent all of its data (and perhaps a little padding), it responds to the next 'Get
// Data' with a disconnect message containing some random bytes.
//
// The sequence of messages is as follows:
//
//			(master)--- ReadClass1230  -->(outstation)  Initiate connection
//			(master)<-- G30V4Q0 + size ---(outstation)  Send Size
//		 Loop:
//			(master)--- ReadClass123   -->(outstation)  Get Data
//			(master)<-- G30V3Q0 + data ---(outstation)  Send Data
//	    ...
//		 End:
//			(master)--- ReadClass123   -->(outstation)  Get Data
//			(master)<-- G30V1Q0 + rand ---(outstation)  Disconnect
package secondary

import (
	"bytes"
	"dingopie/internal"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/nblair2/go-dnp3/dnp3"
)

// ==================================================================
// COMMON
// ==================================================================

var (
	initiateConnection = internal.DNP3ReadClass1230
	sendSize           = [][]byte{internal.DNP3G30V4Q0}
	getData            = internal.DNP3ReadClass123
	sendData           = [][]byte{internal.DNP3G30V3Q0}
	disconnect         = [][]byte{internal.DNP3G30V1Q0}
	sendChan           = make(chan []byte, 1)
	recvChan           = make(chan []byte, 1)
	dataSeq            internal.DataSequence
	frame              dnp3.Frame
)

// ==================================================================
// SERVER SEND
// ==================================================================

// ServerSend - dingopie server direct send.
func ServerSend(ip string, port int, data []byte, objects int) error {
	var err error

	dataSeq, err = internal.NewDataSequence(data, objects)
	if err != nil {
		return fmt.Errorf("error creating data sequence: %w", err)
	}

	frame = internal.NewDNP3ResponseFrame()

	// Open socket, wait for connection
	socket := fmt.Sprintf("%s:%d", ip, port)

	ln, err := net.Listen("tcp", socket)
	if err != nil {
		return fmt.Errorf("error starting TCP listener: %w", err)
	}

	defer ln.Close()

	fmt.Printf(">> Listening on %s\n", socket)

	conn, err := ln.Accept()
	fmt.Printf(">> Client connected: %s\n", conn.RemoteAddr().String())

	if err != nil {
		return fmt.Errorf("error accepting connection: %w", err)
	}

	defer conn.Close()

	// run go funcs
	connErrChan := make(chan error, 1)
	procErrChan := make(chan error, 1)

	go func() { connErrChan <- internal.ServerHandleConn(conn, recvChan, sendChan) }()
	go func() { procErrChan <- serverProcess() }()

	var completed int
	for completed < 2 {
		select {
		case err := <-connErrChan:
			if err != nil {
				return fmt.Errorf("error with connection: %w", err)
			}

			completed++
		case err := <-procErrChan:
			if err != nil {
				return fmt.Errorf("error with processing: %w", err)
			}

			completed++
		}
	}

	return nil
}

func serverProcess() error {
	_, err := internal.ServerExchange(
		&frame,
		initiateConnection,
		sendSize,
		[][]byte{dataSeq.SizeBytes},
		recvChan,
		sendChan,
	)
	if err != nil {
		return fmt.Errorf("error during handshake: %w", err)
	}

	bar := internal.NewProgressBar(dataSeq.OriginalLength, ">>>> Sending: ")

	for _, chunk := range dataSeq.DataChunks {
		_, err := internal.ServerExchange(
			&frame,
			getData,
			sendData,
			[][]byte{chunk},
			recvChan,
			sendChan,
		)
		if err != nil {
			return fmt.Errorf("error sending data packet: %w", err)
		}

		bar.Add(dataSeq.ChunkSize)
	}

	_, err = internal.ServerExchange(
		&frame,
		getData,
		disconnect,
		[][]byte{append([]byte{1}, internal.NewRandomBytes(4)...)},
		recvChan,
		sendChan,
	)
	if err != nil {
		return fmt.Errorf("error during disconnect: %w", err)
	}

	bar.Finish()

	return nil
}

// ==================================================================
// CLIENT RECEIVE
// ==================================================================

type recvResult struct {
	data []byte
	err  error
}

// ClientReceive - dingopie client direct receive.
func ClientReceive(ip string, port int, wait time.Duration) ([]byte, error) {
	frame = internal.NewDNP3RequestFrame()

	conn, err := net.Dial("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("error connecting: %w", err)
	}
	defer conn.Close()

	fmt.Printf(">> Connected to %s:%d\n", ip, port)

	connErrChan := make(chan error, 1)
	procErrChan := make(chan recvResult, 1)

	go func() { connErrChan <- internal.ClientHandleConn(conn, sendChan, recvChan) }()
	go func() { procErrChan <- clientReceiveProcess(wait) }()

	for {
		select {
		case err := <-connErrChan:
			return nil, fmt.Errorf("error with connection: %w", err)
		case result := <-procErrChan:
			return result.data, result.err
		}
	}
}

func clientReceiveProcess(wait time.Duration) recvResult {
	var data []byte

	recvDataSlice, err := internal.ClientExchange(
		&frame,
		initiateConnection,
		sendSize,
		nil,
		sendChan,
		recvChan,
	)
	if err != nil {
		return recvResult{data, fmt.Errorf("error during connect exchange: %w", err)}
	}

	recvData := bytes.Join(recvDataSlice, nil)
	if len(recvData) != 4 {
		return recvResult{data, fmt.Errorf("unexpected size data length: %d", len(recvData))}
	}

	size := int(binary.BigEndian.Uint32(recvData))
	bar := internal.NewProgressBar(size, ">>>> Receiving: ")

	for len(data) < size {
		time.Sleep(wait)

		recvDataSlice, err := internal.ClientExchange(
			&frame,
			getData,
			sendData,
			nil,
			sendChan,
			recvChan,
		)
		if err != nil {
			return recvResult{data, fmt.Errorf("error during get data exchange: %w", err)}
		}

		recvData = bytes.Join(recvDataSlice, nil)
		data = append(data, recvData...)

		bar.Add(len(recvData))
	}

	bar.Finish()

	_, err = internal.ClientExchange(&frame, getData, disconnect, nil, sendChan, recvChan)

	return recvResult{data[:size], err}
}

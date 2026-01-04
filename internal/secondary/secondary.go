// Package secondary is sending data server (outstation) --> client (master). The server waits for client connection.
// When client connects, the server responds with the size(in bytes) of the data to be sent (before padding).
// This size is essential so that the client knows how much of the received data to 'throw out' after the end of
// transfer. After this 'handshake' the client periodically polls the server to 'Get Data'. The interval between these
// polls is configurable, the 'wait' flag. The server responds to each of these requests with a 'chunk' of data.
// The size of these chunks is configurable, the 'points' flag will determine how many 4-byte points to send in each
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
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/nblair2/dingopie/internal"
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
	rxCipher           cipher.Stream
)

// ==================================================================
// SERVER SEND
// ==================================================================

// ServerSend - dingopie server direct send.
func ServerSend(ip string, port int,
	key string,
	data []byte,
	points int, pointVarriance float32,
) error {
	var err error

	pointsLow, pointsHigh := internal.GetPointVarianceRange(points, pointVarriance, 60)

	dataSeq, err = internal.NewDataSequence(key, data, pointsLow, pointsHigh)
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
	if err != nil {
		return fmt.Errorf("error accepting connection: %w", err)
	}
	defer conn.Close()

	fmt.Printf("\tConnection %s\n", conn.RemoteAddr().String())

	// run go funcs
	connErrChan := make(chan error, 1)
	procErrChan := make(chan error, 1)

	go func() { connErrChan <- internal.ServerHandleConn(conn, recvChan, sendChan) }()
	go func() { procErrChan <- serverProcess() }()

	for completed := 0; completed < 2; {
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

			fmt.Println("\tAll data sent, waiting for client to close TCP connection")
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

	bar := internal.NewProgressBar(int(dataSeq.OriginalLength), "\tSending:\t")

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

		bar.Add(len(chunk))
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
func ClientReceive(ip string, port int, key string, wait time.Duration) ([]byte, error) {
	frame = internal.NewDNP3RequestFrame()
	rxCipher = internal.NewCipherStream(key)

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
			fmt.Println(">> Data receive complete, closing TCP connection")

			return result.data, result.err
		}
	}
}

func clientReceiveProcess(wait time.Duration) recvResult {
	var data []byte

	recvDataSlice, err := internal.ClientExchange(
		&frame,
		initiateConnection,
		sendSize, nil,
		sendChan, recvChan,
	)
	if err != nil {
		return recvResult{data, fmt.Errorf("error during connect exchange: %w", err)}
	}

	recvData := bytes.Join(recvDataSlice, nil)
	if len(recvData) != 4 {
		return recvResult{data, fmt.Errorf("unexpected size data length: %d", len(recvData))}
	}

	decSize := make([]byte, len(recvData))
	rxCipher.XORKeyStream(decSize, recvData)
	size := int(binary.BigEndian.Uint32(decSize))
	bar := internal.NewProgressBar(size, "\tReceiving:\t")

	for len(data) < size {
		time.Sleep(wait)

		recvDataSlice, err := internal.ClientExchange(
			&frame,
			getData,
			sendData, nil,
			sendChan, recvChan,
		)
		if err != nil {
			return recvResult{data, fmt.Errorf("error during get data exchange: %w", err)}
		}

		recvData = bytes.Join(recvDataSlice, nil)
		decData := make([]byte, len(recvData))
		rxCipher.XORKeyStream(decData, recvData)
		data = append(data, decData...)
		bar.Add(len(decData))
	}

	bar.Finish()

	_, err = internal.ClientExchange(&frame, getData, disconnect, nil, sendChan, recvChan)

	return recvResult{data[:size], err}
}

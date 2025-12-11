package direct

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/schollz/progressbar/v3"
)

// sendData struct chunks up our data before we send it.
type sendData struct {
	dataChunks     [][]byte
	current        int
	originalLength int
	numChunks      int
	chunkSize      int
}

func newSendData(data []byte, dataLen, objects int) sendData {
	var chunks [][]byte

	chunkSize := objects * 4
	data = padDataToChunkSize(data, chunkSize)

	sizeBytes := make([]byte, 4)
	//nolint:gosec // G115: dataLen clamped in calling function, must be positive and less than 4,294,967,295
	binary.BigEndian.PutUint32(sizeBytes, uint32(dataLen))
	chunks = append(chunks, sizeBytes)

	paddedDataLen := len(data)
	for i := 0; i < paddedDataLen; i += chunkSize {
		end := i + chunkSize
		end = min(end, paddedDataLen)
		chunks = append(chunks, data[i:end])
	}

	return sendData{
		dataChunks:     chunks,
		current:        0,
		originalLength: dataLen,
		numChunks:      len(chunks),
		chunkSize:      chunkSize,
	}
}

// Globas across server and worker go funcs.
var (
	serverSendChan = make(chan []byte)
	serverRecvChan = make(chan []byte)
	serverFrame    = newDNP3ResponseFrame()
	dataSeq        sendData
)

// ServerSend pairs with ClientReceive.
func ServerSend(ip string, port int, data []byte, objects int) error {
	dataLen := len(data)
	if dataLen > 0xFFFFFFFF {
		return fmt.Errorf("data length %d exceeds maximum of 4,294,967,295 bytes", dataLen)
	}

	dataSeq = newSendData(data, dataLen, objects)

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

	go func() { connErrChan <- serverHandleConn(conn) }()
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

// serverHandleConn manages the TCP connection, passing read bytes to serverRecvChan and then writing bytes from
// serverSendChan.
func serverHandleConn(conn net.Conn) error {
	for {
		buf := make([]byte, 4096)

		n, err := conn.Read(buf)
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return fmt.Errorf("error reading from connection: %w", err)
		}

		msg := make([]byte, n)
		copy(msg, buf[:n])

		serverRecvChan <- msg

		resp := <-serverSendChan

		_, err = conn.Write(resp)
		if err != nil {
			return fmt.Errorf("error writing to connection: %w", err)
		}
	}
}

// serverProcess manages the main business logic for the server, waiting for specific signals and responding
// appropriately.
func serverProcess() error {
	err := serverExchange(reqConnect, respSendSize, dataSeq.dataChunks[dataSeq.current])
	if err != nil {
		return fmt.Errorf("error during handshake: %w", err)
	}

	dataSeq.current++

	bar := progressbar.NewOptions(dataSeq.originalLength,
		progressbar.OptionSetDescription(">>>> Sending: "),
		progressbar.OptionSetTheme(progressbar.ThemeASCII),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("bytes"),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
	)

	for {
		err := serverExchange(reqGetData, respSendData, dataSeq.dataChunks[dataSeq.current])
		if err != nil {
			return fmt.Errorf("error sending data packet: %w", err)
		}

		bar.Add(dataSeq.chunkSize)

		dataSeq.current++

		if dataSeq.current >= dataSeq.numChunks {
			break
		}
	}

	randBytes := make([]byte, 4)

	_, err = rand.Read(randBytes)
	if err != nil {
		return fmt.Errorf("error generating random disconnect data: %w", err)
	}

	err = serverExchange(reqGetData, respDisconnect, append([]byte{1}, randBytes...))
	if err != nil {
		return fmt.Errorf("error during disconnect: %w", err)
	}

	bar.Finish()

	return nil
}

func serverExchange(expectedSig, sendSig signal, data []byte) error {
	recvData := <-serverRecvChan

	sig, _, err := getSignalDataFromDNP3Bytes(recvData)
	if err != nil {
		return fmt.Errorf("error getting signal from bytes: %w", err)
	} else if sig != expectedSig {
		return fmt.Errorf("unexpected signal received %d, expected %d", sig, expectedSig)
	}

	msg, err := makeDNP3Bytes(serverFrame, sendSig, data)
	if err != nil {
		return fmt.Errorf("error making dnp3 bytes: %w", err)
	}

	serverSendChan <- msg

	return nil
}

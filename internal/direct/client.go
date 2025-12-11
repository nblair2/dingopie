package direct

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/schollz/progressbar/v3"
)

var (
	clientSendChan = make(chan []byte)
	clientRecvChan = make(chan []byte)
	clientFrame    = newDNP3RequestFrame()
)

type recvResult struct {
	data []byte
	err  error
}

// ClientReceive pairs with ServerSend.
func ClientReceive(ip string, port int, wait time.Duration) ([]byte, error) {
	conn, err := net.Dial("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("error connecting: %w", err)
	}
	defer conn.Close()

	fmt.Printf(">> Connected to %s:%d\n", ip, port)

	connErrChan := make(chan error, 1)
	procErrChan := make(chan recvResult, 1)

	go func() { connErrChan <- clientHandleConn(conn) }()
	go func() { procErrChan <- clientProcess(wait) }()

	for {
		select {
		case err := <-connErrChan:
			return nil, fmt.Errorf("error with connection: %w", err)
		case result := <-procErrChan:
			return result.data, result.err
		}
	}
}

func clientHandleConn(conn net.Conn) error {
	for {
		_, err := conn.Write(<-clientSendChan)
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

		clientRecvChan <- msg
	}
}

func clientProcess(wait time.Duration) recvResult {
	var data []byte

	sig, appDataBytes, err := clientExchange(reqConnect, respSendSize)
	if err != nil {
		return recvResult{data, fmt.Errorf("error during connect exchange: %w", err)}
	}

	if sig != respSendSize {
		return recvResult{data, fmt.Errorf("unexpected signal received %d", sig)}
	}

	size := int(binary.BigEndian.Uint32(appDataBytes))

	bar := progressbar.NewOptions(size,
		progressbar.OptionSetDescription(">>>> Receiving: "),
		progressbar.OptionSetTheme(progressbar.ThemeASCII),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("bytes"),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
	)

	expectedSignal := respSendData

	for {
		time.Sleep(wait)

		sig, appDataBytes, err = clientExchange(reqGetData, expectedSignal)
		if err != nil {
			return recvResult{data, fmt.Errorf("error during get data exchange: %w", err)}
		}

		//nolint:exhaustive // Only handling expected signals, default handles unexpected
		switch sig {
		case respSendData:
			data = append(data, appDataBytes...)
			bar.Add(len(appDataBytes))
			if len(data) >= size {
				expectedSignal = respDisconnect
			}
		case respDisconnect:
			bar.Finish()

			return recvResult{data[:size], nil}
		default:
			return recvResult{data, fmt.Errorf("unexpected signal received %d", sig)}
		}
	}
}

func clientExchange(sendSig, expectedSig signal) (signal, []byte, error) {
	msg, err := makeDNP3Bytes(clientFrame, sendSig, nil)
	if err != nil {
		return nosignal, nil, fmt.Errorf("error making DNP3 bytes: %w", err)
	}

	clientSendChan <- msg

	recvData := <-clientRecvChan

	sig, appDataBytes, err := getSignalDataFromDNP3Bytes(recvData)
	if err != nil {
		return nosignal, nil, fmt.Errorf("error getting signal from DNP3 bytes: %w", err)
	} else if sig != expectedSig {
		return nosignal, nil, fmt.Errorf("unexpected signal received %d, expected %d", sig, expectedSig)
	}

	return sig, appDataBytes, nil
}

func ClientSend(ip string, port int, data []byte, wait time.Duration, objects int) error {
	return nil
}

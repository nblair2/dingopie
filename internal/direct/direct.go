// Package direct mode establishes a new DNP3 connection between a client and waiting server
package direct

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"slices"

	"github.com/nblair2/go-dnp3/dnp3"
)

type signal int

const (
	nosignal signal = iota

	// Request (master to outstation)
	// Secondary (server send, client receive).
	reqConnect
	reqGetData
	// Primary (client send, server receive)
	// reqSendSize
	// reqSendData
	// reqDisconnect.

	// Response (outstation to master)
	// Secondary.
	respSendSize
	respSendData
	respDisconnect
	// Primary
	// respAckConnect
	// respAckDisconnect.
)

var signalAppHeaderMap = map[signal][]byte{
	reqConnect:     dnp3ReadClass1230, // 12 bytes, no data
	reqGetData:     dnp3ReadClass123,  // 9 bytes, no data
	respSendSize:   dnp3G30V4Q0,       // 3 bytes + 2 bytes + N * 2 bytes of data
	respSendData:   dnp3G30V3Q0,       // 3 bytes + 2 bytes + N * 4 bytes of data
	respDisconnect: dnp3G30V1Q0,       // 3 bytes + 2 bytes + N * 5 bytes of 'data'
	// RespAckConnect:    dnp3G30V4Q0,       // 3 bytes + 2 bytes + N * 2 bytes of 'data'
	// ReqSendSize:       dnp3G41V2Q0,       // 3 bytes + 2 bytes + N * 2 bytes of data
	// ReqSendData:       dnp3G41V1Q0,       // 3 bytes + 2 bytes + N * 4 bytes of data
	// ReqDisconnect:     dnp3G41V3Q0,       // 3 bytes + 2 bytes + N * 5 bytes of 'data'
	// RespAckDisconnect: dnp3G30V3Q0,       // 3 bytes + 2 bytes + N * 4 bytes of 'data'
}

var signalPointSizeMap = map[signal]int{
	respSendSize:   2, // 2 bytes per point
	respSendData:   4, // 4 bytes per point
	respDisconnect: 5, // 1 byte flag + 4 bytes data per point
	// RespAckConnect:    2, // 2 bytes per point
	// ReqSendSize:       2, // 2 bytes per point
	// ReqSendData:       4, // 1 byte flag + 4 bytes data per point
	// ReqDisconnect:     5, // 1 byte flag + 4 bytes data per point
	// RespAckDisconnect: 4, // 4 bytes per point
}

// var signalsWithData = []signal{
// 	respSendSize,
// 	respSendData,
// 	respDisconnect,
// 	// respAckConnect,
// 	// reqSendSize,
// 	// reqSendData,
// 	// reqDisconnect,
// 	// respAckDisconnect,
// }

var signalsWithoutData = []signal{
	reqConnect,
	reqGetData,
}

func padDataToChunkSize(data []byte, chunkSize int) []byte {
	padLen := chunkSize - (len(data) % chunkSize)
	padBytes := make([]byte, padLen)
	//nolint:errcheck // Failure is just 0 bytes
	rand.Read(padBytes)

	return append(data, padBytes...)
}

func getSignalDataFromDNP3Bytes(data []byte) (signal, []byte, error) {
	frame := dnp3.Frame{}

	err := frame.FromBytes(data)
	if err != nil {
		return nosignal, nil, err
	}

	app := frame.Application.GetData()

	appData, err := app.ToBytes()
	if err != nil {
		return nosignal, nil, err
	}

	return getSignalDataFromAppBytes(appData)
}

func getSignalDataFromAppBytes(data []byte) (signal, []byte, error) {
	if len(data) == 9 && slices.Equal(data, signalAppHeaderMap[reqGetData]) {
		return reqGetData, nil, nil
	}

	if len(data) == 12 && slices.Equal(data, signalAppHeaderMap[reqConnect]) {
		return reqConnect, nil, nil
	}

	for sig, pattern := range signalAppHeaderMap {
		if slices.Equal(data[:3], pattern) {
			return sig, data[5:], nil
		}
	}

	return nosignal, nil, fmt.Errorf("unknown signal: %v", data)
}

func makeDNP3Bytes(frame dnp3.Frame, sig signal, data []byte) ([]byte, error) {
	appBytes, ok := signalAppHeaderMap[sig]
	if !ok {
		return nil, fmt.Errorf("unknown signal: %v", sig)
	}

	if !slices.Contains(signalsWithoutData, sig) {
		size := len(data) / signalPointSizeMap[sig]

		start, err := rand.Int(rand.Reader, big.NewInt(255-int64(size)))
		if err != nil {
			return nil, fmt.Errorf("error generating random start index: %w", err)
		}

		startInt := int(start.Int64())
		end := startInt + size - 1
		appBytes = append(appBytes, byte(startInt), byte(end))
	}

	appBytes = append(appBytes, data...)

	appData := dnp3.ApplicationData{}

	err := appData.FromBytes(appBytes)
	if err != nil {
		return nil, fmt.Errorf("error creating application data from bytes: %w", err)
	}

	frame.Application.SetData(appData)

	return frame.ToBytes()
}

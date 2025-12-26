package internal

import (
	"errors"
	"fmt"
	"math/rand"
	"slices"

	"github.com/nblair2/go-dnp3/dnp3"
)

// dnp3.go contains DNP3 specific constants and helper functions.

const (
	dnp3MasterAddress     uint16 = 1
	dnp3OutstationAddress uint16 = 1024
)

var (
	// DNP3ReadClass123 object header - 9 bytes, no data.
	DNP3ReadClass123 = []byte{
		0x3C, 0x02, 0x06, // class 1
		0x3C, 0x03, 0x06, // class 2
		0x3C, 0x04, 0x06, // class 3
	}

	// DNP3ReadClass1230 object header - 12 bytes, no data.
	DNP3ReadClass1230 = []byte{
		0x3C, 0x02, 0x06, // class 1
		0x3C, 0x03, 0x06, // class 2
		0x3C, 0x04, 0x06, // class 3
		0x3C, 0x01, 0x06, // class 0
	}

	// DNP3G30V1Q0 object header G30, V1, QF 0 - Analog Input 32 bit with flag.
	DNP3G30V1Q0 = []byte{
		0x1E, // Group 30
		0x01, // Variation 1
		0x00, // Qualifier Fields 0: packed without prefix, 1-octet start and stop indices
		// Start Index
		// Stop Index
		// n * (1 byte flag + 4 bytes of data)
	}

	// DNP3G30V3Q0 object header G30, V3, QF 0 - Analog Input 32 bit without flag.
	DNP3G30V3Q0 = []byte{
		0x1E, // Group 30
		0x03, // Variation 3
		0x00, // Qualifier Fields 0: packed without prefix, 1-octet start and stop indices
		// Start Index
		// Stop Index
		// n * 4 bytes of data
	}

	// DNP3G30V4Q0 object header G30, V4, QF 0 - Analog Input 16 bit without flag.
	DNP3G30V4Q0 = []byte{
		0x1E, // Group 30
		0x04, // Variation 4
		0x00, // Qualifier Fields 0: packed without prefix, 1-octet start and stop indices
		// Start Index
		// Stop Index
		// n * 2 bytes of data
	}

	// DNP3G41V1Q0 object header G41, V1, QF 1 - Analog Output 32 bit.
	DNP3G41V1Q0 = []byte{
		0x29, // Group 41
		0x01, // Variation 1
		0x00, // Qualifier Fields 0: packed without prefix, 1-octet start and stop indices
		// Start Index
		// Stop Index
		// n * 4 bytes of data
	}

	// DNP3G41V2Q0 object header G41, V2, QF 0 - Analog Output 16 bit.
	DNP3G41V2Q0 = []byte{
		0x29, // Group 41
		0x02, // Variation 2
		0x00, // Qualifier Fields 0: packed without prefix, 1-octet start and stop indices
		// Start Index
		// Stop Index
		// n * 3 bytes of data
	}

	// DNP3G41V3Q0 object header G41, V3, QF 1 - Analog Output single precision float with flag.
	DNP3G41V3Q0 = []byte{
		0x29, // Group 41
		0x03, // Variation 3
		0x00, // Qualifier Fields 0: packed without prefix, 1-octet start and stop indices
		// Start Index
		// Stop Index
		// n * (1 byte flag + 4 bytes of data) TODO check this
	}
)

var objectNoDataHeaders = [][]byte{
	DNP3ReadClass123,
	DNP3ReadClass1230,
}

var objectHeaders = [][]byte{
	DNP3G30V4Q0,
	DNP3G30V3Q0,
	DNP3G30V1Q0,
	DNP3G41V2Q0,
	DNP3G41V1Q0,
	DNP3G41V3Q0,
}

var pointSizeMap = map[string]int{
	string(DNP3ReadClass123):  0,
	string(DNP3ReadClass1230): 0,
	string(DNP3G30V4Q0):       2,
	string(DNP3G30V3Q0):       4,
	string(DNP3G30V1Q0):       5, // 1 byte flag + 4 bytes data
	string(DNP3G41V2Q0):       3, // 2 bytes data + 1 byte flag
	string(DNP3G41V1Q0):       5,
	string(DNP3G41V3Q0):       5,
}

func newDNP3Frame(request bool, src, dst uint16) dnp3.Frame {
	frame := dnp3.Frame{
		DataLink: dnp3.DataLink{
			Source:      src,
			Destination: dst,
			Control: dnp3.DataLinkControl{
				Direction:       request,
				Primary:         true,
				FrameCountBit:   false,
				FrameCountValid: false,
				FunctionCode:    dnp3.UnconfirmedUserData,
			},
		},
		Transport: dnp3.Transport{
			Final: true,
			First: true,
			//nolint:gosec // G404: Just need a random sequence, not cryptographically relevant
			Sequence: uint8(rand.Intn(63)),
		},
	}
	if request {
		frame.Application = &dnp3.ApplicationRequest{
			Control: dnp3.ApplicationControl{
				First:       true,
				Final:       true,
				Confirm:     false,
				Unsolicited: false,
				//nolint:gosec // G404: Just need a random sequence, not cryptographically relevant
				Sequence: uint8(rand.Intn(15)),
			},
			FunctionCode: dnp3.Read,
		}
	} else {
		frame.Application = &dnp3.ApplicationResponse{
			Control: dnp3.ApplicationControl{
				First:       true,
				Final:       true,
				Confirm:     false,
				Unsolicited: false,
				//nolint:gosec // G404: Just need a random sequence, not cryptographically relevant
				Sequence: uint8(rand.Intn(15)),
			},
			FunctionCode:        dnp3.Response,
			InternalIndications: dnp3.ApplicationInternalIndications{},
		}
	}

	return frame
}

// NewDNP3RequestFrame creates DNP3 request (master to outstation) frame.
func NewDNP3RequestFrame() dnp3.Frame {
	return newDNP3Frame(true, dnp3MasterAddress, dnp3OutstationAddress)
}

// NewDNP3ResponseFrame creates DNP3 response (outstation to master) frame.
func NewDNP3ResponseFrame() dnp3.Frame {
	return newDNP3Frame(false, dnp3OutstationAddress, dnp3MasterAddress)
}

// GetObjectDataFromDNP3Bytes helps parse raw DNP3 frames into an object header (signal) and its data.
func GetObjectDataFromDNP3Bytes(data []byte) ([]byte, []byte, error) {
	frame := dnp3.Frame{}

	err := frame.FromBytes(data)
	if err != nil {
		return nil, nil, err
	}

	app := frame.Application.GetData()

	appData, err := app.ToBytes()
	if err != nil {
		return nil, nil, err
	}

	for _, objHeader := range objectNoDataHeaders {
		if slices.Equal(objHeader, appData) {
			return objHeader, nil, nil
		}
	}

	for _, objHeader := range objectHeaders {
		if slices.Equal(objHeader, appData[:len(objHeader)]) {
			// TODO +2 is a hack to skip start/stop indices
			return objHeader, appData[len(objHeader)+2:], nil
		}
	}

	return nil, appData, errors.New("unknown DNP3 application object header")
}

// MakeDNP3Bytes helps create raw DNP3 frames from an object header (signal) and its data.
func MakeDNP3Bytes(frame dnp3.Frame, header, data []byte) ([]byte, error) {
	// Increment the sequence numbers
	frame.Transport.Sequence = (frame.Transport.Sequence + 1) % 64
	appControl := frame.Application.GetControl()
	appControl.Sequence = (appControl.Sequence + 1) % 16
	frame.Application.SetControl(appControl)

	// Build the Application Data
	pointSize := pointSizeMap[string(header)]
	if pointSize != 0 {
		size := len(data) / pointSize
		//nolint:gosec // G404: Just need a random number, not cryptographically relevant
		start := rand.Intn(255 - size)
		end := start + size - 1
		header = append(header, byte(start), byte(end))
		header = append(header, data...)
	} else if len(data) > 0 {
		return nil, errors.New("data provided for signal that does not take data")
	}

	appData := dnp3.ApplicationData{}

	err := appData.FromBytes(header)
	if err != nil {
		return nil, fmt.Errorf("error creating application data from bytes: %w", err)
	}

	frame.Application.SetData(appData)

	return frame.ToBytes()
}

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
	// DNP3ReadClass1 object header - 3 bytes, no data.
	DNP3ReadClass1 = []byte{
		0x3C, 0x02, 0x06, // class 1
	}

	// DNP3ReadClass2 object header - 3 bytes, no data.
	DNP3ReadClass2 = []byte{
		0x3C, 0x03, 0x06, // class 2
	}

	// DNP3ReadClass3 object header - 3 bytes, no data.
	DNP3ReadClass3 = []byte{
		0x3C, 0x04, 0x06, // class 3
	}

	// DNP3ReadClass0 object header - 3 bytes, no data.
	DNP3ReadClass0 = []byte{
		0x3C, 0x01, 0x06, // class 0
	}

	// DNP3ReadClass123 object header - 9 bytes, no data.
	DNP3ReadClass123 = [][]byte{
		DNP3ReadClass1,
		DNP3ReadClass2,
		DNP3ReadClass3,
	}

	// DNP3ReadClass1230 object header - 12 bytes, no data.
	DNP3ReadClass1230 = [][]byte{
		DNP3ReadClass1,
		DNP3ReadClass2,
		DNP3ReadClass3,
		DNP3ReadClass0,
	}

	// // DNP3ReadClass123 object header - 9 bytes, no data.
	// DNP3ReadClass123 = slices.Concat(
	// 	DNP3ReadClass1,
	// 	DNP3ReadClass2,
	// 	DNP3ReadClass3,
	// ).

	// // DNP3ReadClass1230 object header - 12 bytes, no data.
	// DNP3ReadClass1230 = slices.Concat(
	// 	DNP3ReadClass1,
	// 	DNP3ReadClass2,
	// 	DNP3ReadClass3,
	// 	DNP3ReadClass0,
	// ).

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
	DNP3ReadClass1,
	DNP3ReadClass2,
	DNP3ReadClass3,
	DNP3ReadClass0,
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
	// string(DNP3ReadClass123):  0,
	// string(DNP3ReadClass1230): 0,
	string(DNP3ReadClass1): 0,
	string(DNP3ReadClass2): 0,
	string(DNP3ReadClass3): 0,
	string(DNP3ReadClass0): 0,
	string(DNP3G30V4Q0):    2,
	string(DNP3G30V3Q0):    4,
	string(DNP3G30V1Q0):    5, // 4 bytes data + 1 byte flag
	string(DNP3G41V2Q0):    3, // 2 bytes data + 1 byte flag
	string(DNP3G41V1Q0):    5,
	string(DNP3G41V3Q0):    5,
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
func GetObjectDataFromDNP3Bytes(inData []byte) ([][]byte, [][]byte, error) {
	var headers, data [][]byte

	frame := dnp3.Frame{}

	err := frame.FromBytes(inData)
	if err != nil {
		return headers, data, fmt.Errorf("error parsing DNP3 frame from bytes: %w", err)
	}

	app := frame.Application.GetData()

ObjectsLoop:
	for _, obj := range app.Objects {
		objData, err := obj.ToBytes()
		if err != nil {
			return headers, data, fmt.Errorf("error converting DNP3 object to bytes: %w", err)
		}

		for _, hdr := range objectNoDataHeaders {
			if slices.Equal(hdr, objData) {
				headers = append(headers, hdr)
				data = append(data, nil)

				continue ObjectsLoop
			}
		}

		for _, hdr := range objectHeaders {
			if slices.Equal(hdr, objData[:len(hdr)]) {
				headers = append(headers, hdr)
				// TODO +2 is a hack to skip start/stop indices
				data = append(data, objData[len(hdr)+2:])

				continue ObjectsLoop
			}
		}

		headers = append(headers, nil)
		data = append(data, objData)

		return headers, data, errors.New("unknown DNP3 application object header")
	}

	return headers, data, nil
}

// SplitDNP3Frames takes in a byte slice of an arbitrary number of concatenated DNP3 frames and splits them into
// individual frames, returned as a slice of byte slices.
// This solves the problem of reading from a socket and getting multiple frames at once.
func SplitDNP3Frames(data []byte) ([][]byte, error) {
	var frames [][]byte

	offset := 0
	for offset < len(data) {
		if data[offset] != 0x05 || data[offset+1] != 0x64 {
			return frames, fmt.Errorf("invalid DNP3 frame start at offset %d", offset)
		}

		length := int(data[offset+2])
		if length < 5 {
			return frames, fmt.Errorf("invalid DNP3 frame length %d at offset %d", length, offset)
		}
		// add crcs to length.
		// 5 more bytes in header not accounted for, and then
		length = 5 + length + 2*((length+10)/16)
		if offset+length > len(data) {
			return frames, fmt.Errorf("incomplete DNP3 frame at offset %d", offset)
		}

		frames = append(frames, data[offset:offset+length])
		offset += length
	}

	return frames, nil
}

// MakeDNP3Bytes helps create raw DNP3 frames from pairs of object headers and data.
func MakeDNP3Bytes(frame *dnp3.Frame, headerDataPairs ...[]byte) ([]byte, error) {
	incrementDNP3Sequence(frame)

	var result []byte

	if len(headerDataPairs)%2 != 0 {
		return nil, errors.New("data slices must be in pairs of header and data")
	}

	for i := 0; i < len(headerDataPairs); i += 2 {
		header := headerDataPairs[i]
		data := headerDataPairs[i+1]
		pointSize := pointSizeMap[string(header)]
		result = append(result, header...)

		if pointSize != 0 {
			if len(data)%pointSize != 0 {
				return nil, fmt.Errorf(
					"data length %d not padded to multiple of %d for object header %v",
					len(data),
					pointSize,
					header,
				)
			}

			size := len(data) / pointSize
			if size > 255 {
				return nil, fmt.Errorf(
					"data length %d results in %d objects, exceeds max of 255 for object header %v",
					len(data),
					size,
					header,
				)
			}
			//nolint:gosec // G404: Just need a random number, not cryptographically relevant
			start := rand.Intn(256 - size)
			end := start + size - 1

			result = append(result, byte(start), byte(end))
			result = append(result, data...)
		} else if len(data) > 0 {
			return nil, errors.New("data provided for signal that does not take data")
		}
	}

	appData := dnp3.ApplicationData{}

	err := appData.FromBytes(result)
	if err != nil {
		return nil, fmt.Errorf("error parsing application data from bytes: %w", err)
	}

	frame.Application.SetData(appData)

	return frame.ToBytes()
}

func incrementDNP3Sequence(frame *dnp3.Frame) {
	frame.Transport.Sequence = (frame.Transport.Sequence + 1) % 64
	appControl := frame.Application.GetControl()
	appControl.Sequence = (appControl.Sequence + 1) % 16
	frame.Application.SetControl(appControl)
}

package direct

import (
	"math/rand"

	"github.com/nblair2/go-dnp3/dnp3"
)

const (
	dnp3MasterAddress     uint16 = 1
	dnp3OutstationAddress uint16 = 1024
)

var (
	// Read Class 1230 - 9 bytes, no data.
	dnp3ReadClass123 = []byte{
		0x3C, 0x02, 0x06, // class 1
		0x3C, 0x03, 0x06, // class 2
		0x3C, 0x04, 0x06, // class 3
	}

	// Read Class 1230 - 12 bytes, no data.
	dnp3ReadClass1230 = []byte{
		0x3C, 0x02, 0x06, // class 1
		0x3C, 0x03, 0x06, // class 2
		0x3C, 0x04, 0x06, // class 3
		0x3C, 0x01, 0x06, // class 0
	}

	// G30, V1, QF 0 - Analog Input 32 bit with flag.
	dnp3G30V1Q0 = []byte{
		0x1E, // Group 30
		0x01, // Variation 1
		0x00, // Qualifier Fields 0: packed without prefix, 1-octet start and stop indices
		// Start Index
		// Stop Index
		// n * (1 byte flag + 4 bytes of data)
	}

	// G30, V3, QF 0 - Analog Input 32 bit without flag.
	dnp3G30V3Q0 = []byte{
		0x1E, // Group 30
		0x03, // Variation 3
		0x00, // Qualifier Fields 0: packed without prefix, 1-octet start and stop indices
		// Start Index
		// Stop Index
		// n * 4 bytes of data
	}

	// G30, V4, QF 0 - Analog Input 16 bit without flag.
	dnp3G30V4Q0 = []byte{
		0x1E, // Group 30
		0x04, // Variation 4
		0x00, // Qualifier Fields 0: packed without prefix, 1-octet start and stop indices
		// Start Index
		// Stop Index
		// n * 2 bytes of data
	}

	// G41, V1, QF 1 - Analog Output 32 bit.
	dnp3G41V1Q0 = []byte{
		0x29, // Group 41
		0x01, // Variation 1
		0x00, // Qualifier Fields 0: packed without prefix, 1-octet start and stop indices
		// Start Index
		// Stop Index
		// n * 4 bytes of data
	}

	// G41, V2, QF 0 - Analog Output 16 bit.
	dnp3G41V2Q0 = []byte{
		0x29, // Group 41
		0x02, // Variation 2
		0x00, // Qualifier Fields 0: packed without prefix, 1-octet start and stop indices
		// Start Index
		// Stop Index
		// n * 2 bytes of data
	}

	// G41, V3, QF 1 - Analog Output single precision float with flag.
	dnp3G41V3Q0 = []byte{
		0x29, // Group 41
		0x03, // Variation 3
		0x00, // Qualifier Fields 0: packed without prefix, 1-octet start and stop indices
		// Start Index
		// Stop Index
		// n * (1 byte flag + 4 bytes of data) TODO check this
	}
)

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

func newDNP3RequestFrame() dnp3.Frame {
	return newDNP3Frame(true, dnp3MasterAddress, dnp3OutstationAddress)
}

func newDNP3ResponseFrame() dnp3.Frame {
	return newDNP3Frame(false, dnp3OutstationAddress, dnp3MasterAddress)
}

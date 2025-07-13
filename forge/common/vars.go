package common

const (
	// DNP3 Data
	MASTER_ADDR     = 1
	OUTSTATION_ADDR = 10
	DL_CTL_FC       = 4    // Unconfirmed user data
	APP_REQ_FC      = 0x01 // Read
	APP_RESP_FC     = 0x81 // Read response

	// Application Object data, these all interalate, be careful
	DNP3_OBJ_HEADER_SIZE = 5 // G30, V3, Q0, see below
	DNP3_OBJ_SIZE        = 4 // 32 bit
)

var (
	// Class 1230 == ask for the length of data
	REQ_SIZE []byte = []byte{
		0x3c, 0x02, 0x06, // class 1
		0x3c, 0x03, 0x06, // class 2
		0x3c, 0x04, 0x06, // class 3
		0x3C, 0x01, 0x06, // class 0
	}

	// Class 123 == ask for the next block of data
	REQ_DATA []byte = []byte{
		0x3c, 0x02, 0x06, // class 1
		0x3c, 0x03, 0x06, // class 2
		0x3c, 0x04, 0x06, // class 3
	}
	// Group 30, Variation 3, Qualifier 0
	RESP_OBJ_HEADER = []byte{
		0x1E, // Group 30
		0x03, // Variaiton 4
		0x00, // Qualifier Fields 0 (its complex...)
		0x00, // Start Index
		// Stop index is the number of chunks (configurable)
	}
)

package forgecommon

const (
	// DNP3 Data.
	DNP3MasterAddress     = 1
	DNP3OutstationAddress = 10
	UnconfirmedUserDataFC = 4    // Unconfirmed user data
	ApplicationRequestFC  = 0x01 // Read
	ApplicationResponseFC = 0x81 // Read response

	// Application Object data, these all interrelate, be careful.
	DNP3ObjHeaderSize = 5 // G30, V3, Q0, see below
	DNP3ObjSize       = 4 // 32 bit
)

var (
	// Class 1230 == ask for the length of data.
	RequestSize = []byte{
		0x3c, 0x02, 0x06, // class 1
		0x3c, 0x03, 0x06, // class 2
		0x3c, 0x04, 0x06, // class 3
		0x3C, 0x01, 0x06, // class 0
	}

	// Class 123 == ask for the next block of data.
	RequestData = []byte{
		0x3c, 0x02, 0x06, // class 1
		0x3c, 0x03, 0x06, // class 2
		0x3c, 0x04, 0x06, // class 3
	}
	// Group 30, Variation 3, Qualifier 0.
	ResponseObjectHeader = []byte{
		0x1E, // Group 30
		0x03, // Variaiton 4
		0x00, // Qualifier Fields 0 (its complex...)
		0x00, // Start Index
		// Stop index is the number of chunks (configurable)
	}
)

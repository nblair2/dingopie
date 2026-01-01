// Package internal contains common helper functions and constants used across the project.
package internal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/schollz/progressbar/v3"
)

// ==================================================================
// "CRYPTO"
// ==================================================================

// NewCipherStream creates a new AES CTR cipher stream using a key derived from the provided password.
func NewCipherStream(password string) cipher.Stream {
	key := sha256.Sum256([]byte(password))
	block, _ := aes.NewCipher(key[:])
	stream := cipher.NewCTR(block, make([]byte, aes.BlockSize))

	return stream
}

// XorData performs XOR encryption/decryption on the input data using a key derived from the provided password.
func XorData(password string, data []byte) []byte {
	stream := NewCipherStream(password)
	out := make([]byte, len(data))
	stream.XORKeyStream(out, data)

	return out
}

// NewRandomBytes generates a slice of random bytes of the specified size.
func NewRandomBytes(size int) []byte {
	b := make([]byte, size)
	//nolint:errcheck // Failure to create random bytes leads to null which is acceptable
	rand.Read(b)

	return b
}

// ==================================================================
// Data
// ==================================================================

// DataSequence struct chunks up our data before we send it.
type DataSequence struct {
	DataChunks     [][]byte // the data, split up into n chunks
	OriginalLength uint32   // the original length of the data before padding
	SizeBytes      []byte   // the original length as a big-endian uint32 (ready to send)
	NumChunks      int      // number of chunks
	ChunkSize      int      // size of each chunk in bytes, should be multiple of 4
}

// NewDataSequence creates a DataSequence from raw data and the number of objects per chunk.
func NewDataSequence(data []byte, objects int) (DataSequence, error) {
	var chunks [][]byte
	// TODO this is hardcoded based on both client send and server send using 4 byte objects
	const objectSize = 4

	// cast to uint64 to check for overflow before continuing
	if uint64(len(data)) > math.MaxUint32 {
		return DataSequence{}, fmt.Errorf(
			"data length %d exceeds maximum of 4,294,967,295 bytes",
			len(data),
		)
	}
	//nolint:gosec // G115 overflow checked above
	dataLen := uint32(len(data))

	sizeBytes := make([]byte, objectSize)
	binary.BigEndian.PutUint32(sizeBytes, dataLen)

	chunkSize := objects * objectSize
	data = PadDataToChunkSize(data, chunkSize)

	paddedDataLen := len(data)
	for i := 0; i < paddedDataLen; i += chunkSize {
		chunks = append(chunks, data[i:i+chunkSize])
	}

	return DataSequence{
		DataChunks:     chunks,
		OriginalLength: dataLen,
		SizeBytes:      sizeBytes,
		NumChunks:      len(chunks),
		ChunkSize:      chunkSize,
	}, nil
}

// PadDataToChunkSize pads data with random bytes to make its length a multiple of chunkSize.
func PadDataToChunkSize(data []byte, chunkSize int) []byte {
	remainder := len(data) % chunkSize
	if remainder == 0 {
		return data
	}

	padLen := chunkSize - remainder

	return append(data, NewRandomBytes(padLen)...)
}

// InsertPeriodicBytes inserts the a slice into the source starting at offset and repeating every period bytes.
// For example: InsertPeriodicBytes([]byte{0x1,0x2,0x3,0x4,0x5,0x6}, []byte{0xA, 0xB}, 2, 2)
// Results in:  []byte{0x1,0x2,0xA,0xB,0x3,0x4,0xA,0xB,0x5,0x6,0xA,0xB}.
func InsertPeriodicBytes(source, insertion []byte, offset, period int) ([]byte, error) {
	if (len(source)-offset)%period != 0 {
		return nil, errors.New("source length minus offset must be multiple of period")
	}

	var result []byte
	if offset > 0 {
		result = append(result, source[:offset]...)
	}

	for i := offset; i <= len(source); i += period {
		result = append(result, insertion...)
		end := min(i+period, len(source))
		result = append(result, source[i:end]...)
	}

	return result, nil
}

// RemovePeriodicBytes undoes the insertions from InsertPeriodicBytes.
// For example: RemovePeriodicBytes([]byte{0x1,0x2,0xA,0xB,0x3,0x4,0xA,0xB,0x5,0x6,0xA,0xB}, 2, 2, 2)
// Results in:  []byte{0x1,0x2,0x3,0x4,0x5,0x6}.
func RemovePeriodicBytes(source []byte, insertLen, offset, period int) ([]byte, error) {
	if offset > len(source) {
		return nil, fmt.Errorf("offset %d is larger than source length %d", offset, len(source))
	}

	result := make([]byte, 0, len(source))
	if offset > 0 {
		result = append(result, source[:offset]...)
	}

	for i := offset; i < len(source); {
		if i+insertLen > len(source) {
			return nil, fmt.Errorf("source ends with incomplete insertion sequence at index %d", i)
		}

		i += insertLen

		end := min(i+period, len(source))
		result = append(result, source[i:end]...)
		i = end
	}

	return result, nil
}

// ==================================================================
// USER INTERFACE
// ==================================================================.

// NewProgressBar returns a progress bar with standardized options.
func NewProgressBar(size int, message string) *progressbar.ProgressBar {
	return progressbar.NewOptions(size,
		progressbar.OptionSetDescription(message),
		progressbar.OptionSetTheme(progressbar.ThemeASCII),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("bytes"),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionOnCompletion(func() { fmt.Println() }),
	)
}

// Banner helps us follow Rule 1: Look cool.
const Banner = `
▓█████▄  ██▓ ███▄    █   ▄████  ▒█████   ██▓███   ██▓▓█████ 
▒██▀ ██▌▓██▒ ██ ▀█   █  ██▒ ▀█▒▒██▒  ██▒▓██░  ██▒▓██▒▓█   ▀ 
░██   █▌▒██▒▓██  ▀█ ██▒▒██░▄▄▄░▒██░  ██▒▓██░ ██▓▒▒██▒▒███   
░▓█▄   ▌░██░▓██▒  ▐▌██▒░▓█  ██▓▒██   ██░▒██▄█▓▒ ▒░██░▒▓█  ▄ 
░▒████▓ ░██░▒██░   ▓██░░▒▓███▀▒░ ████▓▒░▒██▒ ░  ░░██░░▒████▒
 ▒▒▓  ▒ ░▓  ░ ▒░   ▒ ▒  ░▒   ▒ ░ ▒░▒░▒░ ▒▓▒░ ░  ░░▓  ░░ ▒░ ░
 ░ ▒  ▒  ▒ ░░ ░░   ░ ▒░  ░   ░   ░ ▒ ▒░ ░▒ ░      ▒ ░ ░ ░  ░

      |\__/|     This skullduggery brought       ) (
     /     \     to you by the Camp George      ) ( )
    /_.~ ~,_\        West Computer Club       :::::::::
       \@/                                   ~\_______/~

`

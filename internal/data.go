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

	"github.com/schollz/progressbar/v3"
)

// ==================================================================
// "CRYPTO"
// ==================================================================

// XorData performs XOR encryption/decryption on the input data using a key derived from the provided password.
func XorData(password string, data []byte) []byte {
	key := sha256.Sum256([]byte(password))
	block, _ := aes.NewCipher(key[:])
	stream := cipher.NewCTR(block, make([]byte, aes.BlockSize))
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
	OriginalLength int      // the original length of the data before padding
	SizeBytes      []byte   // the original length as a big-endian uint32 (ready to send)
	NumChunks      int      // number of chunks
	ChunkSize      int      // size of each chunk in bytes, should be multiple of 4
}

// NewDataSequence creates a DataSequence from raw data and the number of objects per chunk.
func NewDataSequence(data []byte, objects int) (DataSequence, error) {
	var chunks [][]byte

	dataLen := len(data)
	if dataLen > 0xFFFFFFFF {
		return DataSequence{}, fmt.Errorf(
			"data length %d exceeds maximum of 4,294,967,295 bytes",
			dataLen,
		)
	}

	sizeBytes := make([]byte, 4)

	binary.BigEndian.PutUint32(sizeBytes, uint32(dataLen))

	chunkSize := objects * 4
	data = padDataToChunkSize(data, chunkSize)

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

func padDataToChunkSize(data []byte, chunkSize int) []byte {
	padLen := chunkSize - (len(data) % chunkSize)

	return append(data, NewRandomBytes(padLen)...)
}

// InsertPeriodicBytes inserts the a slice into the source starting at offset and repeating every period bytes.
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

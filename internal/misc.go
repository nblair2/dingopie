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
	"math/big"

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

// NewRandomBytes generates a slice of random bytes of the specified size.
func NewRandomBytes(size int) []byte {
	b := make([]byte, size)
	//nolint:errcheck // Failure to create random bytes leads to null which is acceptable
	rand.Read(b)

	return b
}

// GetPointVarianceRange calculates the low and high range of points based on the given variance.
func GetPointVarianceRange(points int, variance float32, maxPoints int) (int, int) {
	var pointsLow, pointsHigh int

	switch {
	case variance <= 0:
		return points, points
	case variance >= 1:
		return points, max(2*points, maxPoints)
	}

	pointsLow = int(float32(points) * (1 - variance))
	pointsLow = max(pointsLow, 1)
	pointsHigh = int(float32(points) * (1 + variance))
	pointsHigh = min(pointsHigh, maxPoints)

	return pointsLow, pointsHigh
}

func getRandomInt(low, high int) int {
	if low >= high {
		return low
	}

	biggy, _ := rand.Int(rand.Reader, big.NewInt(int64(high-low)))

	return int(biggy.Int64()) + low
}

// ==================================================================
// Data
// ==================================================================

// DataSequence struct chunks up our data before we send it.
type DataSequence struct {
	DataChunks     [][]byte // the data, split up into n chunks
	OriginalLength uint32   // the original length of the data before padding
	SizeBytes      []byte   // the original length as a big-endian uint32 (ready to send)
}

// NewDataSequence creates a DataSequence from raw data and the number of points per chunk.
func NewDataSequence(key string, data []byte, pointsLow, pointsHigh int) (DataSequence, error) {
	// cast to uint64 to check for overflow before continuing
	if uint64(len(data)) > math.MaxUint32 {
		return DataSequence{}, fmt.Errorf(
			"data length %d exceeds maximum of 4,294,967,295 bytes",
			len(data),
		)
	}
	//nolint:gosec // G115 overflow checked above
	dataLen := uint32(len(data))

	// Now that we may continue
	var chunks [][]byte

	const pointsize = 4 // TODO this is hardcoded based on both client send and server send using 4 byte points

	txCipher := NewCipherStream(key)

	sizeBytes := make([]byte, pointsize)
	binary.BigEndian.PutUint32(sizeBytes, dataLen)

	encSizeBytes := make([]byte, pointsize)
	txCipher.XORKeyStream(encSizeBytes, sizeBytes)

	for i := 0; i < len(data); {
		numPoints := getRandomInt(pointsLow, pointsHigh)
		chunkSize := numPoints * pointsize
		end := min(i+chunkSize, len(data))

		chunk := data[i:end]
		if len(chunk) < chunkSize {
			chunk = PadDataToChunkSize(chunk, chunkSize)
		}

		encChunk := make([]byte, len(chunk))
		txCipher.XORKeyStream(encChunk, chunk)
		chunks = append(chunks, encChunk)
		i += chunkSize
	}

	return DataSequence{
		DataChunks:     chunks,
		OriginalLength: dataLen,
		SizeBytes:      encSizeBytes,
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

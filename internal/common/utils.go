package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
)

// XorData performs XOR encryption/decryption on the input data using a key derived from the provided password.
func XorData(password string, data []byte) []byte {
	key := sha256.Sum256([]byte(password))
	block, _ := aes.NewCipher(key[:])
	stream := cipher.NewCTR(block, make([]byte, aes.BlockSize))
	out := make([]byte, len(data))
	stream.XORKeyStream(out, data)

	return out
}

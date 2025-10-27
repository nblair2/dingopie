package forge

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"

	"github.com/nblair2/go-dnp3/dnp3"
)

func xorData(password string, data []byte) []byte {
	key := sha256.Sum256([]byte(password))
	block, _ := aes.NewCipher(key[:])
	stream := cipher.NewCTR(block, make([]byte, aes.BlockSize))
	out := make([]byte, len(data))
	stream.XORKeyStream(out, data)

	return out
}

func updateSequences(d *dnp3.DNP3) {
	d.Transport.SEQ = (d.Transport.SEQ + 1) % 0b00111111

	err := d.Application.SetSequence((d.Application.GetSequence() + 1) % 0b00001111)
	if err != nil {
		panic(err)
	}
}

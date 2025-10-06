package common

import (
	"math/rand"

	"github.com/nblair2/go-dnp3/dnp3"
)

func simpleStringSeed(s string) int64 {
	var seed int64
	for _, c := range s {
		seed += int64(c)
	}

	return seed
}

func XORData(password string, data []byte) []byte {
	rnd := rand.New(rand.NewSource(simpleStringSeed(password)))

	out := make([]byte, len(data))
	for i := range data {
		out[i] = data[i] ^ byte(rnd.Intn(256))
	}

	return out
}

func UpdateSequences(d *dnp3.DNP3) {
	d.Transport.SEQ = (d.Transport.SEQ + 1) % 0b00111111

	err := d.Application.SetSequence((d.Application.GetSequence() + 1) % 0b00001111)
	if err != nil {
		panic(err)
	}
}

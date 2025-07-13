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

	// This one is much more complicated because Application is an interface
	// and I was too lazy to write a GetSequence method
	var aSeq uint8
	switch a := d.Application.(type) {
	case *dnp3.DNP3ApplicationRequest:
		aSeq = a.CTL.SEQ
	case *dnp3.DNP3ApplicationResponse:
		aSeq = a.CTL.SEQ
	}
	d.Application.SetSequence((aSeq + 1) % 0b00001111)
}

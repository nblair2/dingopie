package app

import (
	"fmt"

	"github.com/nblair2/go-dnp3/dnp3"
)

func CreateDNP3Packet() []byte {
	fmt.Println(dnp3.DNP3{}.String())
	return []byte{0x05, 0x64}
}

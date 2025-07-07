package main

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"

	"github.com/nblair2/go-dnp3/dnp3"
)

func RunClient(addr string, port uint16, wait float32) ([]byte, error) {
	var (
		data []byte
		tSeq uint8 = uint8(rand.Intn(63))
		aSeq uint8 = uint8(rand.Intn(15))
	)
	const (
		HEADER_EXTRA = 5 // Assume G30 V3, must match client
		// DNP3 addresses. Should mirror the other side of channel
		SRC uint16 = 1
		dst uint16 = 10
	)

	fmt.Print(">> Starting DNP3 master client\n")

	p, err := createDNP3ApplicationRequest(SRC, dst, tSeq, aSeq)
	if err != nil {
		return nil,
			fmt.Errorf("creating DNP3 Application Request Packet: %w", err)
	}

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil,
			fmt.Errorf("could not connect to to %s:%d: %w", addr, port, err)
	}
	defer conn.Close()
	fmt.Printf(">>>> Connected to %s\n", conn.RemoteAddr())

	buf := make([]byte, 1024)

	for {
		// Update packet
		tSeq = (tSeq + 1) % 0b00111111
		aSeq = (aSeq + 1) % 0b00001111

		p.Transport.SEQ = tSeq
		err = p.Application.SetSequence(aSeq)
		if err != nil {
			fmt.Printf(">>>> Error updating app seq: %v, continuing\n",
				err)
			aSeq = 0
			p.Application.SetSequence(aSeq)
		}

		// Send a knock
		_, err := conn.Write(p.ToBytes())
		if err != nil {
			fmt.Printf(">>>> Error sending bytes %v, continuing\n", err)
		} else {
			fmt.Print(">>>> Sent DNP3ApplicationRequest\n")
		}

		// Read new data
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				fmt.Printf(">> Got EOF from outstation, closing down\n")
				return data, nil
			}
			fmt.Printf(">>>> Error reading bytes %v, continuing\n", err)
		} else if checkDNP3ApplicationResponse(buf[:n]) {
			d := dnp3.DNP3{}
			err = d.DecodeFromBytes(buf[:n])
			if err != nil {
				fmt.Printf(">>>> Error decoding bytes %v, continuing\n", err)
			} else {
				new := d.Application.LayerPayload()
				new = new[HEADER_EXTRA:] // Hack to remove G/V/Q
				data = append(data, new...)
				fmt.Printf(">>>> Received %d bytes\n", len(new))
			}
		} else {
			fmt.Printf(">>>> Got bytes that were not a "+
				"DNP3ApplicationResponse, continuing: 0x % X", buf[:n])
		}

		time.Sleep(time.Duration(wait) * time.Second)
	}
}

func createDNP3ApplicationRequest(src, dst uint16, tSeq, aSeq uint8) (dnp3.DNP3, error) {
	if tSeq > 0b00111111 {
		return dnp3.DNP3{},
			fmt.Errorf("transport sequence number is only 6 bits, got %d",
				tSeq)
	}
	if aSeq > 0b00001111 {
		return dnp3.DNP3{},
			fmt.Errorf("application sequence number is only 4 bits, got %d",
				aSeq)
	}

	// HACK to make legitimate traffic polling classes 1230
	appReqBytes := []byte{
		0x3c, 0x02, 0x06, // class 1
		0x3c, 0x03, 0x06, // class 2
		0x3c, 0x04, 0x06, // class 3
		0x3C, 0x01, 0x06, // class 0
	}

	return dnp3.DNP3{
		DataLink: dnp3.DNP3DataLink{
			CTL: dnp3.DNP3DataLinkControl{
				DIR: true, // master --> outstation
				PRM: true,
				FCB: false,
				FCV: false,
				FC:  4, //Unconfirmed user data
			},
			DST: dst,
			SRC: src,
		},
		Transport: dnp3.DNP3Transport{
			FIN: true,
			FIR: true,
			SEQ: tSeq,
		},
		Application: &dnp3.DNP3ApplicationRequest{
			CTL: dnp3.DNP3ApplicationControl{
				FIR: true,
				FIN: true,
				CON: false,
				UNS: false,
				SEQ: aSeq,
			},
			FC:  0x01, // Read
			Raw: appReqBytes,
		},
	}, nil

}

func checkDNP3ApplicationResponse(b []byte) bool {
	var d dnp3.DNP3
	err := d.DecodeFromBytes(b)
	if err != nil {
		return false
	} else if d.DataLink.SRC != 10 || d.DataLink.DST != 1 {
		return false
	} else if !d.Transport.FIR || !d.Transport.FIN {
		return false
	}

	switch a := d.Application.(type) {
	case *dnp3.DNP3ApplicationResponse:
		if !a.CTL.FIR || !a.CTL.FIN {
			return false
		} else if a.FC != 0x81 {
			return false
		} else {
			return true
		}
	default:
		return false
	}
}

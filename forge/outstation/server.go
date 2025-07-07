package main

import (
	"fmt"
	"io"
	"math/rand"
	"net"

	"github.com/nblair2/go-dnp3/dnp3"
)

func RunServer(port uint16, data []byte, chunks int) error {
	var (
		// expose some of these to user?
		tSeq   uint8 = uint8(rand.Intn(63))
		aSeq   uint8 = uint8(rand.Intn(15))
		offset int   = 0
	)
	const (
		CHUNK_SIZE = 4 // assuming Group 30, Variation 03
		// DNP3 addresses. Should mirror the other side of channel
		SRC uint16 = 10
		DST uint16 = 1
	)

	fmt.Println(">> Starting DNP3 outstation server")

	p, err := createDNP3ApplicationResponse(SRC, DST, tSeq, aSeq)
	if err != nil {
		return fmt.Errorf("creating DNP3 Application Response Packet: %w", err)
	}

	data = padData(data, chunks*CHUNK_SIZE)
	size := len(data)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("error starting server: %w", err)
	}
	defer listener.Close()
	fmt.Printf(">>>> Listening on %s\n", listener.Addr())

	conn, err := listener.Accept()
	if err != nil {
		fmt.Printf(">>>> Error accepting connection: %v, continuing\n", err)
	}
	fmt.Printf(">>>> Connection from %s\n", conn.RemoteAddr())
	defer conn.Close()
	req := make([]byte, 1024)

	for {
		n, err := conn.Read(req)
		if err != nil {
			if err != io.EOF {
				fmt.Printf(
					">>>> Error reading from %s, %v, continuing\n",
					conn.RemoteAddr(), err)
			}

			return fmt.Errorf("connection closed by remote")
		}

		// If correct knock, send back next block of data
		if checkDNP3ApplicationRequest(req[:n]) {

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

			end := offset + chunks*CHUNK_SIZE
			// with padding above this should not need to be checked
			block := data[offset:end]
			objs := createDNP3ApplicationData(block)
			p.Application.SetContents(objs)

			// Send
			_, err = conn.Write(p.ToBytes())
			if err != nil {
				fmt.Printf(">>>> Error sending bytes %v, continuing\n", err)
			} else {
				fmt.Printf(">>>> Sent bytes %d:%d / %d\n", offset, end, size)
				offset = end
			}

		} else {
			fmt.Println(">>>> Error, did not get DNP3ApplicationRequest" +
				"from connection, continuing")
		}

		if offset >= len(data) {
			fmt.Println(">> All data sent, closing down")
			return nil
		}
	}
}

func createDNP3ApplicationResponse(src, dst uint16, transSeq, appSeq uint8) (dnp3.DNP3, error) {
	if transSeq > 0b00111111 {
		return dnp3.DNP3{},
			fmt.Errorf("transport sequence number is only 6 bits, got %d",
				transSeq)
	}
	if appSeq > 0b00001111 {
		return dnp3.DNP3{},
			fmt.Errorf("application sequence number is only 4 bits, got %d",
				appSeq)
	}

	return dnp3.DNP3{
		DataLink: dnp3.DNP3DataLink{
			CTL: dnp3.DNP3DataLinkControl{
				DIR: false, // outstation -> master
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
			SEQ: transSeq,
		},
		Application: &dnp3.DNP3ApplicationResponse{
			CTL: dnp3.DNP3ApplicationControl{
				FIR: true,
				FIN: true,
				CON: false,
				UNS: false,
				SEQ: appSeq,
			},
			FC:  0x81,
			IIN: dnp3.DNP3ApplicationIIN{}, // All IIN set to 0
		},
	}, nil
}

func checkDNP3ApplicationRequest(b []byte) bool {
	var d dnp3.DNP3
	err := d.DecodeFromBytes(b)
	if err != nil {
		return false
	} else if d.DataLink.SRC != 1 || d.DataLink.DST != 10 {
		return false
	} else if !d.Transport.FIR || !d.Transport.FIN {
		return false
	}

	switch a := d.Application.(type) {
	case *dnp3.DNP3ApplicationRequest:
		if !a.CTL.FIR || !a.CTL.FIN {
			return false
		} else if a.FC != 0x01 {
			return false
		} else {
			return true
		}
	default:
		return false
	}
}

func padData(data []byte, chunk int) []byte {
	length := len(data)
	pad := 0
	if length%chunk != 0 {
		pad = chunk - (length % chunk)
	}

	padded := make([]byte, length+pad)
	copy(padded, data)
	return padded
}

// Hard coding for DNP3 G30 V3 (32 bit analog input w/o flag)
// Structure is: 1 byte G, 1 byte V, 1 byte qualifier 2 byte start index, 2 byte stop index,
func createDNP3ApplicationData(data []byte) []byte {
	var o []byte
	const CHUNK_SIZE = 4
	data = padData(data, CHUNK_SIZE)
	stop := (len(data) / CHUNK_SIZE) - 1
	o = append(o, 0x1E)       // Group 30
	o = append(o, 0x03)       // Variation 3
	o = append(o, 0x00)       // Qualifier Fields 0 (its complex...)
	o = append(o, 0x00)       // Start Index
	o = append(o, byte(stop)) // Stop Index
	o = append(o, data...)    // finally our data
	return o
}

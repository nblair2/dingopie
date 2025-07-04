package main

import (
	"fmt"
	"io"
	"net"

	"github.com/nblair2/go-dnp3/dnp3"
)

func RunServer(port uint16, data []byte) error {
	var (
		// expose some of these to user?
		src    uint16 = 10
		dst    uint16 = 1
		tSeq   uint8  = 6
		aSeq   uint8  = 7
		offset int    = 0
		chunks int    = 10
	)
	const CHUNK_SIZE = 4 // assuming G30V2, 32 bit

	fmt.Println(">> Starting DNP3 outstation server")

	p, err := createDNP3ApplicationResponse(src, dst, tSeq, aSeq)
	if err != nil {
		return fmt.Errorf("creating DNP3 Application Response Packet: %w", err)
	}

	data = padData(data, chunks*CHUNK_SIZE)

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
			p.Application.SetContents(block)

			// Send
			_, err = conn.Write(p.ToBytes())
			if err != nil {
				fmt.Printf(">>>> Error sending bytes %v, continuing\n", err)
			} else {
				fmt.Printf(">>>> Sent bytes %d : %d\n", offset, end)
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

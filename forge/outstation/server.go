package main

import (
	"fmt"
	"io"
	"net"

	"github.com/nblair2/go-dnp3/dnp3"
)

func RunServer(port uint16, key string, data []byte) error {
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
		fmt.Errorf("creating DNP3 Application Response Packet: %w", err)
	}

	data = padData(data, chunks*CHUNK_SIZE)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("error starting server: %w", err)
	}
	defer listener.Close()
	fmt.Println(">>>> Listening on %s", listener.Addr())

	conn, err := listener.Accept()
	if err != nil {
		fmt.Println(">>>> Error accepting connection: %v, continuing", err)
	}
	fmt.Println(">>>> Connection from %s", conn.RemoteAddr())
	defer conn.Close()
	req := make([]byte, 1024)

	for {
		n, err := conn.Read(req)
		if err != nil {
			if err != io.EOF {
				fmt.Println(
					">>>> Error reading from %s, %v, continuing",
					conn.RemoteAddr(), err)
			}
			return nil //EOF, close conn
		}

		if checkDNP3ApplicationRequest(req[:n]) {
			end := offset + chunks*CHUNK_SIZE
			// with padding above this should not need to be checked
			s := addDNP3ApplicationData(p, data[offset:end])
			s.Transport.SEQ = tSeq
			s.Application.CTL.SEQ = aSeq
			_, err := conn.Write(s.ToBytes())
			if err != nil {
				fmt.Println(">>>> Error sending bytes %v, continuing", err)
			}
			fmt.Println(">>>> Sent %d bytes", chunks*CHUNK_SIZE)
			offset = end
			tSeq = (tSeq + 1) % 0b00111111
			aSeq = (aSeq + 1) % 0b00001111
		}

		if offset >= len(data) {
			fmt.Println(">>>> All data sent, closing down")
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
				DIR: false,
				PRM: true,
				FCB: true,
				FCV: true,
				FC:  4,
			},
			DST: src,
			SRC: dst,
		},
		Transport: dnp3.DNP3Transport{
			FIN: true,
			FIR: true,
			SEQ: transSeq,
		},
		Application: dnp3.DNP3Application{
			CTL: dnp3.DNP3ApplicationControl{
				FIR: true,
				FIN: true,
				CON: false,
				UNS: false,
				SEQ: appSeq,
			},
			FC: 0x81,
			IIN: dnp3.DNP3ApplicationIIN{
				AllStations:      false,
				Class1Events:     false,
				Class2Events:     false,
				Class3Events:     false,
				NeedTime:         false,
				Local:            false,
				DeviceTrouble:    false,
				Restart:          false,
				BadFunction:      false,
				ObjectUnknown:    false,
				ParameterError:   false,
				BufferOverflow:   false,
				AlreadyExiting:   false,
				BadConfiguration: false,
				Reserved1:        false,
				Reserved2:        false,
			},
		},
	}, nil
}

func checkDNP3ApplicationRequest(data []byte) bool {
	if len(data) > 5 {
		return true
	}
	return false
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

func addDNP3ApplicationData(dnp3 dnp3.DNP3, data []byte) dnp3.DNP3 {
	dnp3.Application.OBJ = data
	return dnp3
}

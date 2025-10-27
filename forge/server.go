package forge

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"

	"github.com/nblair2/go-dnp3/dnp3"
	"github.com/schollz/progressbar/v3"
)

type Server struct {
	resp     dnp3.DNP3
	listener net.Listener
	conn     net.Conn
}

func NewServer(port uint16) (*Server, error) {
	server := &Server{}
	server.resp = newApplicationResponse()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("error starting server: %w", err)
	}

	server.listener = listener

	fmt.Printf(">> Listening on %s\n", listener.Addr().String())

	conn, err := server.listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("error accepting connection: %w", err)
	}

	server.conn = conn
	fmt.Printf(">> New connection from %s\n", conn.RemoteAddr().String())

	return server, nil
}

//nolint:cyclop,funlen
func (s *Server) RunServer(data []byte, chunk int) error {
	offset := 0
	size := len(data)
	data = padData(data, chunk)

	req := make([]byte, 1024)

	// bar := progressbar.Default(int64(size), ">> Sending data:")
	bar := progressbar.NewOptions(size,
		progressbar.OptionSetDescription(">> Sending data:"),
		progressbar.OptionSetTheme(progressbar.ThemeASCII),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("bits"),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
	)

	for {
		num, err := s.conn.Read(req)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				fmt.Printf(">>>> Error reading from %s, %v, (continuing)\n",
					s.conn.RemoteAddr(), err)
			} else {
				return errors.New("connection closed by remote")
			}
		}

		d, err := decodeRequest(req[:num])
		if err != nil {
			fmt.Printf(">>>> could not decode request: %v (continuing)\n", err)
		}

		appData := d.Application.GetData()

		payload, err := (&appData).ToBytes()
		if err != nil {
			fmt.Printf(">>>> could not encode request data: %v (continuing)\n", err)

			continue
		}

		switch {
		case bytes.Equal(payload, RequestSize):
			err := s.SendData(binary.LittleEndian.AppendUint64(nil, uint64(size)))
			if err != nil {
				fmt.Printf(">>>> Error sending size: %v (continuing)\n", err)

				continue
			}
		case bytes.Equal(payload, RequestData):
			err := s.SendData(data[offset : offset+chunk])
			if err != nil {
				fmt.Printf(">>>> Error sending data: %v (continuing)\n", err)

				continue
			}

			offset += chunk
			_ = bar.Add(chunk)
		default:
			fmt.Println(">>>> request type unknown (continuing)")
		}

		if offset >= len(data) {
			err := bar.Finish()
			if err != nil {
				fmt.Printf(">> failed to finish progress bar: %v (continuing)\n", err)
			}

			fmt.Println("\n>> Closing server")

			return nil
		}
	}
}

func (s *Server) Close() error {
	if s.conn != nil {
		err := s.conn.Close()
		if err != nil {
			return fmt.Errorf("error closing connection: %w", err)
		}
	}

	if s.listener != nil {
		err := s.listener.Close()
		if err != nil {
			return fmt.Errorf("error closing listener: %w", err)
		}
	}

	return nil
}

func (s *Server) SendData(data []byte) error {
	updateSequences(&s.resp)

	num := (len(data) / DNP3ObjSize) - 1
	appData := dnp3.ApplicationData{}

	err := appData.FromBytes(append(append(ResponseObjectHeader, byte(num)), data...))
	if err != nil {
		return fmt.Errorf("could not decode response data: %w", err)
	}

	s.resp.Application.SetData(appData)

	raw, err := s.resp.ToBytes()
	if err != nil {
		return fmt.Errorf("error encoding response: %w", err)
	}

	_, err = s.conn.Write(raw)
	if err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}

	return nil
}

func newApplicationResponse() dnp3.DNP3 {
	return dnp3.DNP3{
		DataLink: dnp3.DataLink{
			CTL: dnp3.DataLinkCTL{
				DIR: false, // outstation -> master
				PRM: true,
				FCB: false,
				FCV: false,
				FC:  UnconfirmedUserDataFC,
			},
			DST: DNP3MasterAddress,
			SRC: DNP3OutstationAddress,
		},
		Transport: dnp3.Transport{
			FIN: true,
			FIR: true,
			//nolint:gosec // G404
			SEQ: uint8(rand.Intn(63)),
		},
		Application: &dnp3.ApplicationResponse{
			CTL: dnp3.ApplicationCTL{
				FIR: true,
				FIN: true,
				CON: false,
				UNS: false,
				//nolint:gosec // G404
				SEQ: uint8(rand.Intn(15)),
			},
			FC:  ApplicationResponseFC,
			IIN: dnp3.ApplicationIIN{}, // All IIN set to 0
		},
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

//nolint:cyclop
func decodeRequest(b []byte) (*dnp3.DNP3, error) {
	var dnp dnp3.DNP3

	err := dnp.FromBytes(b)
	switch {
	case err != nil:
		return nil, fmt.Errorf("could not decode: %w", err)
	case dnp.DataLink.SRC != DNP3MasterAddress ||
		dnp.DataLink.DST != DNP3OutstationAddress:
		return nil, errors.New("got wrong src/dst")
	case !dnp.Transport.FIR || !dnp.Transport.FIN:
		return nil, errors.New("transport not first and last")
	case dnp.DataLink.CTL.FC != UnconfirmedUserDataFC:
		return nil, fmt.Errorf("data link function code is not %#x, got %#x",
			UnconfirmedUserDataFC, dnp.DataLink.CTL.FC)
	case dnp.Application == nil:
		return nil, errors.New("no application layer")
	case dnp.Application.GetFunctionCode() != ApplicationRequestFC:
		return nil, fmt.Errorf("app function code is not %#x, got %#x",
			ApplicationRequestFC, dnp.Application.GetFunctionCode())
	case !dnp.Application.GetCTL().FIR || !dnp.Application.GetCTL().FIN:
		return nil, errors.New("app not first and last")
	}

	if _, ok := dnp.Application.(*dnp3.ApplicationRequest); ok {
		return &dnp, nil
	}

	return nil, errors.New("not a DNP3 Application Request")
}

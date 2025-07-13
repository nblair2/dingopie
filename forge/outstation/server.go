package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"

	"github.com/nblair2/go-dnp3/dnp3"
	"github.com/schollz/progressbar/v3"

	"dingopie/forge/common"
)

type Server struct {
	resp     dnp3.DNP3
	listener net.Listener
	conn     net.Conn
}

func NewServer(port uint16) (*Server, error) {
	s := &Server{}
	s.resp = newApplicationResponse()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("error starting server: %w", err)
	}
	s.listener = listener

	conn, err := s.listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("error accepting connection: %v", err)
	}
	s.conn = conn

	return s, nil
}

func (s *Server) RunServer(data []byte, chunk int) error {
	var offset int = 0
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
		progressbar.OptionClearOnFinish(),
	)

	for {
		n, err := s.conn.Read(req)
		if err != nil {
			if err != io.EOF {
				fmt.Printf(">>>> Error reading from %s, %v, (continuing)\n",
					s.conn.RemoteAddr(), err)
			} else {
				return fmt.Errorf("connection closed by remote")
			}
		}

		d, err := decodeRequest(req[:n])
		if err != nil {
			fmt.Printf(">>>> could not decode request: %v (continuing)\n", err)
		}

		p := d.Application.LayerPayload()
		if bytes.Equal(p, common.REQ_SIZE) {
			err := s.sendData(binary.LittleEndian.AppendUint64(nil, uint64(size)))
			if err != nil {
				fmt.Printf(">>>> Error sending size: %v (continuing)\n", err)
				continue
			}
		} else if bytes.Equal(p, common.REQ_DATA) {
			err := s.sendData(data[offset : offset+chunk])
			if err != nil {
				fmt.Printf(">>>> Error sending data: %v (continuing)\n", err)
				continue
			}
			offset += chunk
			bar.Add(chunk)
		} else {
			fmt.Println(">>>> request type unknown (continuing)")
		}

		if offset >= len(data) {
			bar.Finish()
			fmt.Println(">> Sent all data")
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

func (s *Server) sendData(data []byte) error {
	common.UpdateSequences(&s.resp)

	num := (len(data) / common.DNP3_OBJ_SIZE) - 1
	out := append(common.RESP_OBJ_HEADER, byte(num))
	out = append(out, data...)

	s.resp.Application.SetContents(out)
	_, err := s.conn.Write(s.resp.ToBytes())
	if err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}
	return nil
}

func newApplicationResponse() dnp3.DNP3 {
	return dnp3.DNP3{
		DataLink: dnp3.DNP3DataLink{
			CTL: dnp3.DNP3DataLinkControl{
				DIR: false, // outstation -> master
				PRM: true,
				FCB: false,
				FCV: false,
				FC:  common.DL_CTL_FC,
			},
			DST: common.MASTER_ADDR,
			SRC: common.OUTSTATION_ADDR,
		},
		Transport: dnp3.DNP3Transport{
			FIN: true,
			FIR: true,
			SEQ: uint8(rand.Intn(63)),
		},
		Application: &dnp3.DNP3ApplicationResponse{
			CTL: dnp3.DNP3ApplicationControl{
				FIR: true,
				FIN: true,
				CON: false,
				UNS: false,
				SEQ: uint8(rand.Intn(15)),
			},
			FC:  common.APP_RESP_FC,
			IIN: dnp3.DNP3ApplicationIIN{}, // All IIN set to 0
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

func decodeRequest(b []byte) (*dnp3.DNP3, error) {
	var d dnp3.DNP3
	err := d.DecodeFromBytes(b)
	if err != nil {
		return nil, fmt.Errorf("could not decode: %w", err)
	} else if d.DataLink.SRC != common.MASTER_ADDR ||
		d.DataLink.DST != common.OUTSTATION_ADDR {
		return nil, fmt.Errorf("got wrong src/dst")
	} else if !d.Transport.FIR || !d.Transport.FIN {
		return nil, fmt.Errorf("transport not first and last")
	} else if d.DataLink.CTL.FC != common.DL_CTL_FC {
		return nil, fmt.Errorf("data link function code is not %#x, got %#x",
			common.DL_CTL_FC, d.DataLink.CTL.FC)
	}

	switch a := d.Application.(type) {
	case *dnp3.DNP3ApplicationRequest:
		if !a.CTL.FIR || !a.CTL.FIN {
			return nil, fmt.Errorf("app not first and last")
		} else if a.FC != common.APP_REQ_FC {
			return nil, fmt.Errorf("app function code is not %#x, got %#x",
				common.APP_REQ_FC, a.FC)
		}
		return &d, nil
	}

	return nil, fmt.Errorf("not a DNP3 Application Response")
}

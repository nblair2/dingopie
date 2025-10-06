package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net"

	"dingopie/common"
	"github.com/nblair2/go-dnp3/dnp3"
)

type Client struct {
	req  dnp3.DNP3
	conn net.Conn
}

func NewClient(addr string, port uint16) (*Client, error) {
	c := &Client{}
	c.req = newApplicationRequest()

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, fmt.Errorf("could not connect to %s:%d: %w",
			addr, port, err)
	}

	c.conn = conn

	fmt.Printf(">> Connected to %s\n", conn.RemoteAddr().String())

	return c, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

func (c *Client) GetData(reqAppData []byte) ([]byte, error) {
	buf := make([]byte, 1024)

	common.UpdateSequences(&c.req)

	reqApp := dnp3.ApplicationData{}

	err := reqApp.FromBytes(reqAppData)
	if err != nil {
		return nil, fmt.Errorf("could not decode reqAppData: %w", err)
	}

	c.req.Application.SetData(reqApp)

	reqBytes, err := c.req.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("error encoding req to bytes: %w", err)
	}

	_, err = c.conn.Write(reqBytes)
	if err != nil {
		return nil, fmt.Errorf("error sending req: %w", err)
	}

	n, err := c.conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading resp: %w", err)
	}

	d, err := decodeResponse(buf[:n])
	if err != nil {
		return nil, fmt.Errorf("got invalid response: %w", err)
	}

	appResp := d.Application.GetData()

	appRespData, err := (&appResp).ToBytes()
	if err != nil {
		return nil, fmt.Errorf("could not encode response data: %w", err)
	}

	appRespData = appRespData[common.DNP3_OBJ_HEADER_SIZE:] // Hack to remove G/V/Q

	return appRespData, nil
}

func newApplicationRequest() dnp3.DNP3 {
	return dnp3.DNP3{
		DataLink: dnp3.DataLink{
			CTL: dnp3.DataLinkCTL{
				DIR: true, // master --> outstation
				PRM: true,
				FCB: false,
				FCV: false,
				FC:  common.DL_CTL_FC, // Unconfirmed user data
			},
			DST: common.OUTSTATION_ADDR,
			SRC: common.MASTER_ADDR,
		},
		Transport: dnp3.Transport{
			FIN: true,
			FIR: true,
			SEQ: uint8(rand.Intn(63)),
		},
		Application: &dnp3.ApplicationRequest{
			CTL: dnp3.ApplicationCTL{
				FIR: true,
				FIN: true,
				CON: false,
				UNS: false,
				SEQ: uint8(rand.Intn(15)),
			},
			FC: common.APP_REQ_FC, // Read
			// Raw: []byte{}
		},
	}
}

func decodeResponse(b []byte) (*dnp3.DNP3, error) {
	var d dnp3.DNP3

	err := d.FromBytes(b)
	if err != nil {
		return nil, fmt.Errorf("could not decode from bytes: %w", err)
	} else if d.DataLink.SRC != common.OUTSTATION_ADDR ||
		d.DataLink.DST != common.MASTER_ADDR {
		return nil, errors.New("got wrong src/dst")
	} else if !d.Transport.FIR || !d.Transport.FIN {
		return nil, errors.New("transport not first and last")
	} else if d.DataLink.CTL.FC != common.DL_CTL_FC {
		return nil, fmt.Errorf("data link function code is not %#x, got %#x",
			common.DL_CTL_FC, d.DataLink.CTL.FC)
	}

	switch a := d.Application.(type) {
	case *dnp3.ApplicationResponse:
		if !a.CTL.FIR || !a.CTL.FIN {
			return nil, errors.New("app not first and last")
		} else if a.FC != common.APP_RESP_FC {
			return nil, fmt.Errorf("app function code is not %#x, got %#x",
				common.APP_RESP_FC, a.FC)
		}

		return &d, nil
	}

	return nil, errors.New("not a DNP3 Application Response")
}

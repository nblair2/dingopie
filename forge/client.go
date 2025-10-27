package forge

import (
	"errors"
	"fmt"
	"math/rand"
	"net"

	"github.com/nblair2/go-dnp3/dnp3"
)

type Client struct {
	req  dnp3.DNP3
	conn net.Conn
}

func NewClient(addr string, port uint16) (*Client, error) {
	client := &Client{}
	client.req = createApplicationRequest()

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, fmt.Errorf("could not connect to %s:%d: %w",
			addr, port, err)
	}

	client.conn = conn

	fmt.Printf(">> Connected to %s\n", conn.RemoteAddr().String())

	return client, nil
}

func (client *Client) Close() error {
	if client.conn != nil {
		err := client.conn.Close()
		if err != nil {
			return fmt.Errorf("error closing connection: %w", err)
		}
	}

	return nil
}

func (client *Client) GetData(reqAppData []byte) ([]byte, error) {
	buf := make([]byte, 1024)

	updateSequences(&client.req)

	reqApp := dnp3.ApplicationData{}

	err := reqApp.FromBytes(reqAppData)
	if err != nil {
		return nil, fmt.Errorf("could not decode reqAppData: %w", err)
	}

	client.req.Application.SetData(reqApp)

	reqBytes, err := client.req.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("error encoding req to bytes: %w", err)
	}

	_, err = client.conn.Write(reqBytes)
	if err != nil {
		return nil, fmt.Errorf("error sending req: %w", err)
	}

	n, err := client.conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading resp: %w", err)
	}

	d, err := parseResponse(buf[:n])
	if err != nil {
		return nil, fmt.Errorf("got invalid response: %w", err)
	}

	appResp := d.Application.GetData()

	appRespData, err := (&appResp).ToBytes()
	if err != nil {
		return nil, fmt.Errorf("could not encode response data: %w", err)
	}

	appRespData = appRespData[DNP3ObjHeaderSize:] // Hack to remove G/V/Q

	return appRespData, nil
}

func createApplicationRequest() dnp3.DNP3 {
	return dnp3.DNP3{
		DataLink: dnp3.DataLink{
			CTL: dnp3.DataLinkCTL{
				DIR: true, // master --> outstation
				PRM: true,
				FCB: false,
				FCV: false,
				FC:  UnconfirmedUserDataFC, // Unconfirmed user data
			},
			DST: DNP3OutstationAddress,
			SRC: DNP3MasterAddress,
		},
		Transport: dnp3.Transport{
			FIN: true,
			FIR: true,
			//nolint:gosec // G404
			SEQ: uint8(rand.Intn(63)),
		},
		Application: &dnp3.ApplicationRequest{
			CTL: dnp3.ApplicationCTL{
				FIR: true,
				FIN: true,
				CON: false,
				UNS: false,
				//nolint:gosec // G404
				SEQ: uint8(rand.Intn(15)),
			},
			FC: ApplicationRequestFC, // Read
			// Raw: []byte{}
		},
	}
}

//nolint:cyclop
func parseResponse(b []byte) (*dnp3.DNP3, error) {
	var dnp dnp3.DNP3

	err := dnp.FromBytes(b)
	switch {
	case err != nil:
		return nil, fmt.Errorf("could not decode from bytes: %w", err)
	case dnp.DataLink.SRC != DNP3OutstationAddress ||
		dnp.DataLink.DST != DNP3MasterAddress:
		return nil, errors.New("got wrong src/dst")
	case !dnp.Transport.FIR || !dnp.Transport.FIN:
		return nil, errors.New("transport not first and last")
	case dnp.DataLink.CTL.FC != UnconfirmedUserDataFC:
		return nil, fmt.Errorf("data link function code is not %#x, got %#x",
			UnconfirmedUserDataFC, dnp.DataLink.CTL.FC)
	}

	if a, ok := dnp.Application.(*dnp3.ApplicationResponse); ok {
		if !a.CTL.FIR || !a.CTL.FIN {
			return nil, errors.New("app not first and last")
		} else if a.FC != ApplicationResponseFC {
			return nil, fmt.Errorf("app function code is not %#x, got %#x",
				ApplicationResponseFC, a.FC)
		}

		return &dnp, nil
	}

	return nil, errors.New("not a DNP3 Application Response")
}

package internal_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nblair2/dingopie/internal"
	"github.com/nblair2/go-dnp3/v3/dnp3"
)

// makeFrame builds a single DNP3 frame from headerDataPairs using a fresh
// request frame. Test helper — fails the test on construction error.
func makeFrame(t *testing.T, headerDataPairs ...[]byte) []byte {
	t.Helper()

	frame := internal.NewDNP3RequestFrame()

	bytes, err := internal.MakeDNP3Bytes(&frame, headerDataPairs...)
	if err != nil {
		t.Fatalf("makeFrame: %v", err)
	}

	return bytes
}

func TestSplitDNP3Frames(t *testing.T) {
	t.Parallel()

	singleFrame := makeFrame(t, internal.DNP3ReadClass1, nil)
	twoFrames := append(append([]byte{}, singleFrame...), singleFrame...)

	cases := []struct {
		name      string
		input     []byte
		wantCount int
		wantErr   string
	}{
		{
			name:      "empty input returns no frames",
			input:     []byte{},
			wantCount: 0,
		},
		{
			name:      "single valid frame",
			input:     singleFrame,
			wantCount: 1,
		},
		{
			name:      "two concatenated valid frames",
			input:     twoFrames,
			wantCount: 2,
		},
		{
			name:    "bad start bytes returns error",
			input:   []byte{0x00, 0x00, 0x05, 0xC4, 0x01, 0x00, 0x00, 0x04, 0xE9, 0x21},
			wantErr: "magic bytes",
		},
		{
			name:    "declared length below minimum",
			input:   []byte{0x05, 0x64, 0x04, 0xC4, 0x01, 0x00, 0x00, 0x04, 0xE9, 0x21},
			wantErr: "invalid DNP3 length",
		},
		{
			name:    "declared length runs past buffer end",
			input:   singleFrame[:len(singleFrame)-3],
			wantErr: "incomplete DNP3 frame",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			frames, err := internal.SplitDNP3Frames(tc.input)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}

				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(frames) != tc.wantCount {
				t.Fatalf("got %d frames, want %d", len(frames), tc.wantCount)
			}

			// Each returned slice must itself parse as a valid DNP3 frame.
			for i, f := range frames {
				_, parseErr := dnp3.NewFrameFromBytes(f)
				if parseErr != nil {
					t.Errorf("frame[%d] does not parse: %v", i, parseErr)
				}
			}
		})
	}
}

func TestGetObjectDataFromDNP3Bytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		headerPairs [][]byte
		wantHeaders [][]byte
		wantData    [][]byte
		wantErr     string
	}{
		{
			name:        "single no-data header (DNP3ReadClass1)",
			headerPairs: [][]byte{internal.DNP3ReadClass1, nil},
			wantHeaders: [][]byte{internal.DNP3ReadClass1},
			wantData:    [][]byte{nil},
		},
		{
			name: "single data header with one analog point (DNP3G30V3Q0)",
			headerPairs: [][]byte{
				internal.DNP3G30V3Q0, {0xDE, 0xAD, 0xBE, 0xEF},
			},
			wantHeaders: [][]byte{internal.DNP3G30V3Q0},
			wantData:    [][]byte{{0xDE, 0xAD, 0xBE, 0xEF}},
		},
		{
			name: "two no-data headers in same frame",
			headerPairs: [][]byte{
				internal.DNP3ReadClass1, nil,
				internal.DNP3ReadClass2, nil,
			},
			wantHeaders: [][]byte{internal.DNP3ReadClass1, internal.DNP3ReadClass2},
			wantData:    [][]byte{nil, nil},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			frameBytes := makeFrame(t, tc.headerPairs...)

			gotHeaders, gotData, err := internal.GetObjectDataFromDNP3Bytes(frameBytes)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}

				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.wantHeaders, gotHeaders); diff != "" {
				t.Errorf("headers mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantData, gotData); diff != "" {
				t.Errorf("data mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetObjectDataFromDNP3Bytes_ParseError(t *testing.T) {
	t.Parallel()

	// Truncated header bytes — too short to be a frame.
	_, _, err := internal.GetObjectDataFromDNP3Bytes([]byte{0x05, 0x64})
	if err == nil {
		t.Fatal("expected error for truncated frame, got nil")
	}
}

func TestMakeDNP3Bytes_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		headerPairs [][]byte
		wantErr     string
	}{
		{
			name:        "odd number of arguments",
			headerPairs: [][]byte{internal.DNP3ReadClass1},
			wantErr:     "must be in pairs",
		},
		{
			name: "unknown header",
			headerPairs: [][]byte{
				{0xFF, 0xFF, 0x00}, nil,
			},
			wantErr: "unknown object header",
		},
		{
			name: "data length not a multiple of point size",
			headerPairs: [][]byte{
				// DNP3G30V3Q0 has a 4-byte point size; 3 bytes is invalid.
				internal.DNP3G30V3Q0, {0x01, 0x02, 0x03},
			},
			wantErr: "not padded to multiple of",
		},
		{
			name: "data provided for no-data header",
			headerPairs: [][]byte{
				internal.DNP3ReadClass1, {0x01, 0x02},
			},
			wantErr: "signal that does not take data",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			frame := internal.NewDNP3RequestFrame()

			_, err := internal.MakeDNP3Bytes(&frame, tc.headerPairs...)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}

			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestMakeDNP3Bytes_RoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		headerPairs [][]byte
		wantHeaders [][]byte
		wantData    [][]byte
	}{
		{
			name:        "single read-class request",
			headerPairs: [][]byte{internal.DNP3ReadClass1, nil},
			wantHeaders: [][]byte{internal.DNP3ReadClass1},
			wantData:    [][]byte{nil},
		},
		{
			name: "G30V3 with one 4-byte point",
			headerPairs: [][]byte{
				internal.DNP3G30V3Q0, {0x11, 0x22, 0x33, 0x44},
			},
			wantHeaders: [][]byte{internal.DNP3G30V3Q0},
			wantData:    [][]byte{{0x11, 0x22, 0x33, 0x44}},
		},
		{
			name: "G30V3 with two 4-byte points",
			headerPairs: [][]byte{
				internal.DNP3G30V3Q0,
				{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88},
			},
			wantHeaders: [][]byte{internal.DNP3G30V3Q0},
			wantData:    [][]byte{{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}},
		},
		{
			name: "G30V4 with two 2-byte points",
			headerPairs: [][]byte{
				internal.DNP3G30V4Q0, {0xAA, 0xBB, 0xCC, 0xDD},
			},
			wantHeaders: [][]byte{internal.DNP3G30V4Q0},
			wantData:    [][]byte{{0xAA, 0xBB, 0xCC, 0xDD}},
		},
		{
			name: "all four read-class headers in one frame",
			headerPairs: [][]byte{
				internal.DNP3ReadClass1, nil,
				internal.DNP3ReadClass2, nil,
				internal.DNP3ReadClass3, nil,
				internal.DNP3ReadClass0, nil,
			},
			wantHeaders: [][]byte{
				internal.DNP3ReadClass1,
				internal.DNP3ReadClass2,
				internal.DNP3ReadClass3,
				internal.DNP3ReadClass0,
			},
			wantData: [][]byte{nil, nil, nil, nil},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			frame := internal.NewDNP3RequestFrame()

			encoded, err := internal.MakeDNP3Bytes(&frame, tc.headerPairs...)
			if err != nil {
				t.Fatalf("MakeDNP3Bytes: %v", err)
			}

			gotHeaders, gotData, err := internal.GetObjectDataFromDNP3Bytes(encoded)
			if err != nil {
				t.Fatalf("GetObjectDataFromDNP3Bytes: %v", err)
			}

			if diff := cmp.Diff(tc.wantHeaders, gotHeaders); diff != "" {
				t.Errorf("headers mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantData, gotData); diff != "" {
				t.Errorf("data mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewDNP3RequestFrame(t *testing.T) {
	t.Parallel()

	frame := internal.NewDNP3RequestFrame()

	if frame.DataLink.Source != 1 {
		t.Errorf("Source = %d, want 1 (master)", frame.DataLink.Source)
	}

	if frame.DataLink.Destination != 1024 {
		t.Errorf("Destination = %d, want 1024 (outstation)", frame.DataLink.Destination)
	}

	if !frame.DataLink.Control.Direction {
		t.Error("Direction should be true for request (master->outstation)")
	}

	if _, ok := frame.Application.(*dnp3.ApplicationRequest); !ok {
		t.Errorf("Application is %T, want *dnp3.ApplicationRequest", frame.Application)
	}
}

func TestNewDNP3ResponseFrame(t *testing.T) {
	t.Parallel()

	frame := internal.NewDNP3ResponseFrame()

	if frame.DataLink.Source != 1024 {
		t.Errorf("Source = %d, want 1024 (outstation)", frame.DataLink.Source)
	}

	if frame.DataLink.Destination != 1 {
		t.Errorf("Destination = %d, want 1 (master)", frame.DataLink.Destination)
	}

	if frame.DataLink.Control.Direction {
		t.Error("Direction should be false for response (outstation->master)")
	}

	if _, ok := frame.Application.(*dnp3.ApplicationResponse); !ok {
		t.Errorf("Application is %T, want *dnp3.ApplicationResponse", frame.Application)
	}
}

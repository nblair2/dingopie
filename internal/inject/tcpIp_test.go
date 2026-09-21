//go:build !windows

package inject

import (
	"errors"
	"testing"
)

// baseIPv4TCPPacket returns a minimal well-formed 40-byte IPv4 (20-byte header, no
// options) / TCP (20-byte header, no options) packet with an empty payload.
func baseIPv4TCPPacket() []byte {
	pkt := make([]byte, 40)
	pkt[0] = 0x45 // version 4, IHL 5 (20 bytes)
	pkt[9] = 6    // protocol TCP

	// TCP header starts at offset 20; data offset field (upper nibble of byte 12) = 5 (20 bytes).
	pkt[20+12] = 0x50

	return pkt
}

func TestFindIPv4TCPHeader_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		pkt          []byte
		wantSentinel error
	}{
		{
			name:         "too short for IPv4 header",
			pkt:          make([]byte, 10),
			wantSentinel: ErrPacketTooShortIPv4Header,
		},
		{
			name: "not IPv4",
			pkt: func() []byte {
				pkt := baseIPv4TCPPacket()
				pkt[0] = 0x65 // version 6

				return pkt
			}(),
			wantSentinel: ErrNotIPv4,
		},
		{
			name: "not TCP",
			pkt: func() []byte {
				pkt := baseIPv4TCPPacket()
				pkt[9] = 17 // UDP

				return pkt
			}(),
			wantSentinel: ErrNotTCP,
		},
		{
			name: "too short for TCP header",
			pkt: func() []byte {
				pkt := baseIPv4TCPPacket()

				return pkt[:25] // IP header (20) + 5 bytes, not enough for a 20-byte TCP header
			}(),
			wantSentinel: ErrPacketTooShortTCPHeader,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := findIPv4TCPHeader(tc.pkt)
			if !errors.Is(err, tc.wantSentinel) {
				t.Errorf("errors.Is(err, %v) = false, err: %v", tc.wantSentinel, err)
			}
		})
	}
}

func TestFindIPv4TCPHeader_Success(t *testing.T) {
	t.Parallel()

	pkt := baseIPv4TCPPacket()

	ipHdrLen, err := findIPv4TCPHeader(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ipHdrLen != 20 {
		t.Errorf("ipHdrLen = %d, want 20", ipHdrLen)
	}
}

func TestFindDNP3InIPPacket_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		pkt          []byte
		wantSentinel error
	}{
		{
			name:         "underlying IPv4/TCP error propagates",
			pkt:          make([]byte, 10),
			wantSentinel: ErrPacketTooShortIPv4Header,
		},
		{
			name:         "too short for DNP3 magic bytes",
			pkt:          baseIPv4TCPPacket(), // 40 bytes total, nothing past the TCP header
			wantSentinel: ErrPacketTooShortDNP3Magic,
		},
		{
			name: "DNP3 magic bytes not found",
			pkt: func() []byte {
				pkt := baseIPv4TCPPacket()

				return append(pkt, 0x00, 0x00)
			}(),
			wantSentinel: ErrNoDNP3Magic,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := findDNP3InIPPacket(tc.pkt)
			if !errors.Is(err, tc.wantSentinel) {
				t.Errorf("errors.Is(err, %v) = false, err: %v", tc.wantSentinel, err)
			}
		})
	}
}

func TestFindDNP3InIPPacket_Success(t *testing.T) {
	t.Parallel()

	pkt := append(baseIPv4TCPPacket(), 0x05, 0x64)

	ipHdrLen, tcpHdrLen, err := findDNP3InIPPacket(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ipHdrLen != 20 || tcpHdrLen != 20 {
		t.Errorf("ipHdrLen, tcpHdrLen = %d, %d, want 20, 20", ipHdrLen, tcpHdrLen)
	}
}

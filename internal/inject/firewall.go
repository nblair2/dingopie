//go:build !windows

package inject

import (
	"fmt"
	"strconv"

	"github.com/coreos/go-iptables/iptables"
)

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

// newSendAckRule intercepts incoming ACKs from the remote (PREROUTING) so that
// the local TCP stack sees ACK values that match its unmodified sequence space.
// Uses the same nfqueue as the data rule so both directions share one handler.
func newSendAckRule(local, remote string, localPort, remotePort int) *FirewallRule {
	return newRecvRule(local, remote, localPort, remotePort)
}

// newRecvAckRule intercepts outgoing ACKs toward the remote (POSTROUTING) so
// that the sender's TCP stack sees ACK values that include injected bytes.
// Uses the same nfqueue as the data rule so both directions share one handler.
func newRecvAckRule(local, remote string, localPort, remotePort int) *FirewallRule {
	return newSendRule(local, remote, localPort, remotePort)
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

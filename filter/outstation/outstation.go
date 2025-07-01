package outstation

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coreos/go-iptables/iptables"
	nfqueue "github.com/florianl/go-nfqueue"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const (
	TABLE     string = "mangle"
	CHAIN     string = "OUTPUT"
	RULE_NUM  int    = 1
	QUEUE_NUM uint16 = 1
)

func AddIPTableRule(rule ...string) error {
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("error creating new iptables: %w", err)
	}

	err = ipt.Insert(TABLE, CHAIN, RULE_NUM, rule...)
	if err != nil {
		return fmt.Errorf("error inserting rule: %w", err)
	}

	return nil
}

func DeleteIPTableRule(rule ...string) error {
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("error creating new iptables: %w", err)
	}

	err = ipt.DeleteIfExists(TABLE, CHAIN, rule...)
	if err != nil {
		return fmt.Errorf("error deleting rule: %w", err)
	}

	return nil
}

func Send(data []byte, rule []string) error {
	var (
		nf  *nfqueue.Nfqueue
		err error
	)

	//SETUP
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	rule = append(rule, "--jump", "NFQUEUE",
		"--queue-num", fmt.Sprintf("%d", QUEUE_NUM))
	if err = AddIPTableRule(rule...); err != nil {
		return fmt.Errorf("error creating the iptables rule: %w", err)
	}

	config := nfqueue.Config{
		NfQueue:      QUEUE_NUM,
		MaxPacketLen: 0xFFFF,
		MaxQueueLen:  0xFF,
		Copymode:     nfqueue.NfQnlCopyPacket,
		WriteTimeout: time.Second,
	}

	if nf, err = nfqueue.Open(&config); err != nil {
		return fmt.Errorf("could not open nfqueue socket: %w", err)
	}
	defer nf.Close()

	// MODIFY
	var i int = 0
	modFn := func(a nfqueue.Attribute) int {
		if err := Modify(a, nf, data, i); err != nil {
			fmt.Printf("Got an error durring modification: %s", err)
			return 1
		}
		return 0
	}

	errFn := func(e error) int {
		return 0 // Do nothing for now
	}

	if err = nf.RegisterWithErrorFunc(ctx, modFn, errFn); err != nil {
		return fmt.Errorf("error registering modification function: %w", err)
	}

	// WAIT
	select {

	case <-ctx.Done():
		fmt.Println("Send finished successfully")

	case sig := <-sigChan:
		fmt.Printf("\nSend canceled with signal %v\n", sig)
		cancel()
	}

	// CLEAN UP
	fmt.Print("Cleaning up...")
	if err = DeleteIPTableRule(rule...); err != nil {
		return fmt.Errorf(`error deleting the iptables rule,
			you should manually run the command:
				(iptables -t %s -c %s -D %d)
			error recieved: %w`,
			TABLE, CHAIN, RULE_NUM, err)
	}
	time.Sleep(time.Second)
	fmt.Println("done!")

	return nil
}

func Modify(a nfqueue.Attribute, n *nfqueue.Nfqueue, d []byte, i int) error {
	pkt := gopacket.NewPacket(*a.Payload, layers.LayerTypeIPv4, gopacket.Default)
	fmt.Println(pkt)
	// Check to see if the packet has a DNP3 application response

	// Pass packet on
	n.SetVerdict(*a.PacketID, nfqueue.NfAccept)
	return nil
}

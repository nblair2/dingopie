package internal_test

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/nblair2/dingopie/internal"
)

func listenTCP(t *testing.T) (net.Listener, int) {
	t.Helper()

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { ln.Close() })

	addr, err := netip.ParseAddrPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	return ln, int(addr.Port())
}

func TestDialTCPDirect(t *testing.T) {
	t.Parallel()

	_, port := listenTCP(t)

	conn, err := internal.DialTCP("127.0.0.1", port, "")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
}

func TestDialTCPSOCKS5(t *testing.T) {
	t.Parallel()

	const socks5 = "socks5"

	for _, tc := range []struct {
		name     string
		scheme   string
		auth     bool
		rejected bool
	}{
		{name: "remote DNS", scheme: socks5},
		{name: "socks5h alias", scheme: "socks5h"},
		{name: "authentication", scheme: socks5, auth: true},
		{name: "proxy rejection", scheme: socks5, rejected: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ln, port := listenTCP(t)
			proxyURL := tc.scheme + "://" + ln.Addr().String()
			greeting := []byte{5, 1, 0}
			method := byte(0)

			if tc.auth {
				proxyURL = tc.scheme + "://user:p%40ss@" + ln.Addr().String()
				greeting = []byte{5, 2, 0, 2}
				method = 2
			}

			const targetHost = "server.invalid"

			host, targetPort := targetHost, 20000
			request := append([]byte{5, 1, 0, 3, byte(len(targetHost))}, []byte(targetHost)...)
			reply := byte(0)

			if tc.rejected {
				// A reachable destination ensures an accidental direct fallback fails this test.
				host, targetPort = "127.0.0.1", port
				request = []byte{5, 1, 0, 1, 127, 0, 0, 1}
				reply = 5
			}

			request = append(request, byte(targetPort>>8), byte(targetPort&0xff))

			done := make(chan struct{})

			go func() {
				defer close(done)

				conn, err := ln.Accept()
				if err != nil {
					t.Error(err)

					return
				}
				defer conn.Close()

				err = conn.SetDeadline(time.Now().Add(2 * time.Second))
				if err != nil {
					t.Error(err)

					return
				}

				steps := []struct{ read, write []byte }{
					{greeting, []byte{5, method}},
				}
				if tc.auth {
					steps = append(steps, struct{ read, write []byte }{
						[]byte{1, 4, 'u', 's', 'e', 'r', 4, 'p', '@', 's', 's'}, []byte{1, 0},
					})
				}

				steps = append(steps, struct{ read, write []byte }{
					request, []byte{5, reply, 0, 1, 127, 0, 0, 1, 0, 0},
				})
				if !tc.rejected {
					steps = append(
						steps,
						struct{ read, write []byte }{[]byte("ping"), []byte("pong")},
					)
				}

				for _, step := range steps {
					got := make([]byte, len(step.read))

					_, err = io.ReadFull(conn, got)
					if err != nil {
						t.Error(err)

						return
					}

					if !bytes.Equal(got, step.read) {
						t.Errorf("request = %v, want %v", got, step.read)

						return
					}

					_, err = conn.Write(step.write)
					if err != nil {
						t.Error(err)

						return
					}
				}
			}()

			t.Cleanup(func() {
				ln.Close()
				<-done
			})

			conn, err := internal.DialTCP(host, targetPort, proxyURL)
			if conn != nil {
				defer conn.Close()
			}

			if tc.rejected {
				if err == nil {
					t.Error("expected proxy rejection, got a connection")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			_, err = conn.Write([]byte("ping"))
			if err != nil {
				t.Fatal(err)
			}

			got, err := io.ReadAll(conn)
			if err != nil {
				t.Fatal(err)
			}

			if string(got) != "pong" {
				t.Errorf("response = %q, want pong", got)
			}
		})
	}
}

func TestDialTCPInvalidProxy(t *testing.T) {
	t.Parallel()

	for _, proxyURL := range []string{"%", "socks5://", "http://127.0.0.1:1080", "socks4://127.0.0.1:1080"} {
		t.Run(proxyURL, func(t *testing.T) {
			t.Parallel()

			_, port := listenTCP(t)

			conn, err := internal.DialTCP("127.0.0.1", port, proxyURL)
			if conn != nil {
				conn.Close()
			}

			if err == nil {
				t.Fatal("expected invalid proxy error, got a connection")
			}
		})
	}
}

func TestSendMessage_HeaderDataMismatch(t *testing.T) {
	t.Parallel()

	sendChan := make(chan []byte, 1)

	err := internal.SendMessage(
		nil,
		[][]byte{internal.DNP3ReadClass1, internal.DNP3ReadClass2},
		[][]byte{nil},
		sendChan,
	)
	if !errors.Is(err, internal.ErrHeaderDataMismatch) {
		t.Errorf("errors.Is(err, internal.ErrHeaderDataMismatch) = false, err: %v", err)
	}
}

func TestReceiveAndValidate_UnexpectedHeaderCount(t *testing.T) {
	t.Parallel()

	recvChan := make(chan []byte, 1)
	recvChan <- makeFrame(t, internal.DNP3ReadClass1, nil)

	_, err := internal.ReceiveAndValidate(
		recvChan,
		[][]byte{internal.DNP3ReadClass1, internal.DNP3ReadClass2},
	)
	if !errors.Is(err, internal.ErrUnexpectedHeaderCount) {
		t.Errorf("errors.Is(err, internal.ErrUnexpectedHeaderCount) = false, err: %v", err)
	}
}

func TestReceiveAndValidate_UnexpectedSignal(t *testing.T) {
	t.Parallel()

	recvChan := make(chan []byte, 1)
	recvChan <- makeFrame(t, internal.DNP3ReadClass1, nil)

	_, err := internal.ReceiveAndValidate(recvChan, [][]byte{internal.DNP3ReadClass2})
	if !errors.Is(err, internal.ErrUnexpectedSignal) {
		t.Errorf("errors.Is(err, internal.ErrUnexpectedSignal) = false, err: %v", err)
	}
}

// TestClientHandleConn_ConnectionClosed verifies that when the remote end of the
// connection closes, ClientHandleConn returns an error matchable both as
// internal.ErrConnectionClosed and (via wrapping) io.EOF.
func TestClientHandleConn_ConnectionClosed(t *testing.T) {
	t.Parallel()

	client, remote := net.Pipe()

	write := make(chan []byte, 1)
	read := make(chan []byte, 1)

	write <- []byte{0x01}

	done := make(chan error, 1)

	go func() {
		done <- internal.ClientHandleConn(client, write, read)
	}()

	// Drain the write, then close our end so the next client.Read sees EOF.
	buf := make([]byte, 16)

	_, err := remote.Read(buf)
	if err != nil {
		t.Fatalf("remote.Read: %v", err)
	}

	err = remote.Close()
	if err != nil {
		t.Fatalf("remote.Close: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, internal.ErrConnectionClosed) {
			t.Errorf("errors.Is(err, internal.ErrConnectionClosed) = false, err: %v", err)
		}

		if !errors.Is(err, io.EOF) {
			t.Errorf("errors.Is(err, io.EOF) = false, err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ClientHandleConn to return")
	}
}

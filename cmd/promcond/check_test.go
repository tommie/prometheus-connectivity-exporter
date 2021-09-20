package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/go-ping/ping"
	"github.com/tommie/chargen2p"
)

func TestRunCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runCheck(ctx, ConnectivityCheck{Kind: KindHostPing, Network: "ip", Host: "localhost", Interval: 10 * time.Millisecond}, waitChecker{done: cancel}, 0)
}

func TestDoCheck(t *testing.T) {
	ctx := context.Background()

	t.Run("ping", func(t *testing.T) {
		var chkr fakeChecker
		if err := doCheck(ctx, &ConnectivityCheck{Kind: KindHostPing, Network: "ip", Host: "localhost"}, &chkr); err != nil {
			t.Fatalf("doCheck failed: %v", err)
		}

		if want := 1; chkr.NumPingCalls != want {
			t.Errorf("NumPingCalls: got %d, want %d", chkr.NumPingCalls, want)
		}
	})

	t.Run("floodping", func(t *testing.T) {
		var chkr fakeChecker
		if err := doCheck(ctx, &ConnectivityCheck{Kind: KindHostFloodPing, Network: "ip", Host: "localhost"}, &chkr); err != nil {
			t.Fatalf("doCheck failed: %v", err)
		}

		if want := 1; chkr.NumPingCalls != want {
			t.Errorf("NumPingCalls: got %d, want %d", chkr.NumPingCalls, want)
		}
	})

	t.Run("connect", func(t *testing.T) {
		var chkr fakeChecker
		if err := doCheck(ctx, &ConnectivityCheck{Kind: KindConnect, Network: "ip", Host: "localhost", Service: "echo"}, &chkr); err != nil {
			t.Fatalf("doCheck failed: %v", err)
		}

		if want := 1; chkr.NumConnectCalls != want {
			t.Errorf("NumConnectCalls: got %d, want %d", chkr.NumConnectCalls, want)
		}
	})

	t.Run("transfer", func(t *testing.T) {
		var chkr fakeChecker
		if err := doCheck(ctx, &ConnectivityCheck{Kind: KindTransfer, Network: "ip", Host: "localhost", Service: "echo"}, &chkr); err != nil {
			t.Fatalf("doCheck failed: %v", err)
		}

		if want := 1; chkr.NumTransferCalls != want {
			t.Errorf("NumTransferCalls: got %d, want %d", chkr.NumTransferCalls, want)
		}
	})
}

func TestCheckPing(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Ping test requires privileges CircleCI/Docker doesn't provide")
	}

	ctx := context.Background()

	pi := pingInterval
	pingInterval = 10 * time.Millisecond
	defer func() {
		pingInterval = pi
	}()

	got, err := checker{}.CheckPing(ctx, "ip", "localhost", false)
	if err != nil {
		t.Fatalf("CheckPing failed: %v", err)
	}

	if got.PacketsSent == 0 {
		t.Errorf("CheckPing PacketsSent: got %v, want >0", got.PacketsSent)
	}
	if got.PacketsRecv != got.PacketsSent {
		t.Errorf("CheckPing PacketsRecv: got %v, want %v", got.PacketsRecv, got.PacketsSent)
	}
	if got.AvgRtt == 0 {
		t.Errorf("CheckPing AvgRtt: got %v, want >0", got.AvgRtt)
	}
}

func TestCheckConnect(t *testing.T) {
	ctx := context.Background()

	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	go func() {
		defer l.Close()

		conn, err := l.Accept()
		if err != nil {
			t.Fatalf("Accept failed: %v", err)
		}
		conn.Close()
	}()

	taddr := l.Addr().(*net.TCPAddr)

	got, err := checker{}.CheckConnect(ctx, "ip", taddr.IP.String(), fmt.Sprint(taddr.Port))
	if err != nil {
		t.Fatalf("CheckConnect failed: %v", err)
	}

	if got == 0 {
		t.Errorf("CheckConnect: got %v, want >0", got)
	}
}

func TestCheckTransfer(t *testing.T) {
	ctx := context.Background()

	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	go func() {
		if err := acceptAndEcho(l); err != nil {
			t.Fatalf("acceptAndEcho failed: %v", err)
		}
	}()

	taddr := l.Addr().(*net.TCPAddr)

	got, got2, got3, err := checker{}.checkTransfer(ctx, "ip", taddr.IP.String(), fmt.Sprint(taddr.Port), chargen2p.WithTolerance(0.9))
	if err != nil {
		t.Fatalf("CheckTransfer failed: %v", err)
	}

	if got == 0 {
		t.Errorf("CheckTransfer: got %v, want >0", got)
	}
	if got2 == 0 {
		t.Errorf("CheckTransfer got2: got %v, want >0", got2)
	}
	if got3 == 0 {
		t.Errorf("CheckTransfer got3: got %v, want >0", got3)
	}
}

func acceptAndEcho(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if errors.Is(err, net.ErrClosed) {
			return nil
		} else if err != nil {
			return err
		}

		bs, err := io.ReadAll(conn)
		if err != nil {
			conn.Close()
			return err
		}
		if _, err := conn.Write(bs); err != nil {
			conn.Close()
			return err
		}

		conn.Close()
	}
}

type fakeChecker struct {
	NumPingCalls     int
	NumConnectCalls  int
	NumTransferCalls int
}

func (c *fakeChecker) CheckPing(ctx context.Context, network, host string, flood bool) (*ping.Statistics, error) {
	c.NumPingCalls++
	return &ping.Statistics{AvgRtt: 1 * time.Second, PacketLoss: 0.5}, nil
}
func (c *fakeChecker) CheckConnect(ctx context.Context, network, host, service string) (time.Duration, error) {
	c.NumConnectCalls++
	return 2 * time.Second, nil
}
func (c *fakeChecker) CheckTransfer(ctx context.Context, network, host, service string) (nbytes int, dur time.Duration, dialDur time.Duration, err error) {
	c.NumTransferCalls++
	return 1024, 4 * time.Second, 3 * time.Second, nil
}

func (*fakeChecker) Resolver() netResolver {
	return defaultResolver
}

type waitChecker struct {
	Checker

	done func()
}

func (c waitChecker) CheckPing(ctx context.Context, network, host string, flood bool) (*ping.Statistics, error) {
	c.done()
	return &ping.Statistics{}, nil
}

func (waitChecker) Resolver() netResolver {
	return defaultResolver
}

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/tommie/chargen2p"
)

func TestCheckPing(t *testing.T) {
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

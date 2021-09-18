package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-ping/ping"
	"github.com/tommie/chargen2p"
)

// pingInterval sets the interval for KindHostPing. It's a test
// injection point.
var pingInterval = 1 * time.Second

type checker struct{}

// CheckPing runs a few ICMP pings to the host. If "flood", it runs a few
// hundred pings to measure packet loss with reasonable accuracy.
func (checker) CheckPing(ctx context.Context, network, host string, flood bool) (*ping.Statistics, error) {
	p := ping.New(host)
	p.SetNetwork(network)
	p.SetPrivileged(false)
	if flood {
		p.Count = 200
		p.Interval = 10 * time.Millisecond
	} else {
		p.Count = 3
		p.Interval = pingInterval
	}
	p.Timeout = time.Duration(p.Count*10) * p.Interval
	p.RecordRtts = false // We only use aggregates.
	stopCh := make(chan struct{})
	defer close(stopCh)
	go func() {
		select {
		case <-ctx.Done():
			p.Stop()
		case <-stopCh:
			// break
		}
	}()
	if err := p.Run(); err != nil {
		return nil, err
	}
	return p.Statistics(), nil
}

// CheckConnect performs a connection handshake and returns how long it took.
func (checker) CheckConnect(ctx context.Context, network, host, service string) (time.Duration, error) {
	network = transportForNetwork(network, KindConnect)
	var d net.Dialer
	start := time.Now()
	conn, err := d.DialContext(ctx, network, host+":"+service)
	if err != nil {
		return 0, err
	}
	end := time.Now()
	conn.Close()

	return end.Sub(start), nil
}

// CheckTransfer sends and receives stream data to measure throughput.
func (c checker) CheckTransfer(ctx context.Context, network, host, service string) (nbytes int, dur time.Duration, dialDur time.Duration, err error) {
	return c.checkTransfer(ctx, network, host, service)
}

func (checker) checkTransfer(ctx context.Context, network, host, service string, opts ...chargen2p.MeasureThroughputOpt) (nbytes int, dur time.Duration, dialDur time.Duration, err error) {
	network = transportForNetwork(network, KindTransfer)
	ti, err := chargen2p.MeasureThroughput(ctx, network, host+":"+service, opts...)
	if err != nil {
		return 0, 0, 0, err
	}
	return ti.NumReadBytes, ti.ReadDuration, ti.DialDuration, nil
}

// transportForNetwork returns the appropriate transport-layer
// "network" string for a given network-layer "network" string, as
// required by the kind of check.
func transportForNetwork(network string, kind ConnectivityCheckKind) string {
	s := "udp"
	switch kind {
	case KindConnect, KindTransfer:
		s = "tcp"
	}
	switch network {
	case "ip4":
		s += "4"
	case "ip6":
		s += "6"
	}
	return s
}

type ConnectivityCheckKind int

const (
	UnknownKind ConnectivityCheckKind = iota

	// KindHostPing sends a few pings and determines the RTT. No
	// packet loss figure will be reported. This implies TypeDatagram.
	KindHostPing

	// KindHostFloodPing sends many pings (quickly) and determines
	// both RTT and packet loss accurately. This implies TypeDatagram.
	KindHostFloodPing

	// KindConnect performs a connect on a stream socket, and reports
	// how long the handshake took.
	KindConnect

	// KindTransfer runs an "echo" test over a stream socket and
	// reports data transfer speeds. This requires an "echo" server on
	// the other end.
	KindTransfer
)

func parseConnectivityCheckKind(s string) (ConnectivityCheckKind, error) {
	switch s {
	case "ping":
		return KindHostPing, nil
	case "flood":
		return KindHostFloodPing, nil
	case "connect":
		return KindConnect, nil
	case "transfer":
		return KindTransfer, nil
	default:
		return UnknownKind, fmt.Errorf("unknown connectivity check kind: %s", s)
	}
}

func (k ConnectivityCheckKind) String() string {
	switch k {
	case UnknownKind:
		return "unknown"
	case KindHostPing:
		return "ping"
	case KindHostFloodPing:
		return "flood"
	case KindConnect:
		return "connect"
	case KindTransfer:
		return "transfer"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/go-ping/ping"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tommie/chargen2p"
)

var (
	// pingInterval sets the interval for KindHostPing. It's a test
	// injection point.
	pingInterval = 1 * time.Second

	checkFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "connectivity",
		Name:      "check_failures",
		Help:      "Failures during checks.",
	}, []string{"af", "host", "service", "kind"})

	// In this case, reporting the ratio itself is probably
	// right. I can't see that we'd want this weighted by number
	// of packages rather than by host.
	hostPacketLoss = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "connectivity",
		Name:      "host_packet_loss",
		Help:      "Packet loss between instance and remote host.",
	}, []string{"af", "host"})
	hostRTT = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "connectivity",
		Name:      "host_rtt",
		Help:      "RTT between instance and remote host.",
	}, []string{"af", "host"})
	serviceLatency = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "connectivity",
		Name:      "service_latency",
		Help:      "Latency between the instance and a remote service.",
	}, []string{"af", "host", "service", "kind"})
	serviceThroughput = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "connectivity",
		Name:      "service_throughput",
		Help:      "Whether the instance can use a remote service.",
	}, []string{"af", "host", "service", "kind"})
)

func init() {
	prometheus.MustRegister(checkFailures)
	prometheus.MustRegister(hostPacketLoss)
	prometheus.MustRegister(hostRTT)
	prometheus.MustRegister(serviceLatency)
	prometheus.MustRegister(serviceThroughput)
}

func startChecks(ctx context.Context, checks []ConnectivityCheck, chkr Checker) {
	// We want data as early as possible, but avoid overlapping
	// checks. The randomization domain depends on the number of
	// checks and how slow they are, not the check's interval.
	for _, chk := range checks {
		go runCheck(ctx, chk, chkr, time.Duration(rand.Intn(int(10*time.Second)*len(checks))))
	}
}

// ConnectivityCheck encapsulates a single check against a host or service on a host.
type ConnectivityCheck struct {
	Kind    ConnectivityCheckKind
	Network string
	Host    string
	Service string

	Interval time.Duration
}

// A Checker is used by startChecks to do the actual checking.
type Checker interface {
	CheckPing(ctx context.Context, network, host string, flood bool) (*ping.Statistics, error)
	CheckConnect(ctx context.Context, network, host, service string) (time.Duration, error)
	CheckTransfer(ctx context.Context, network, host, service string) (nbytes int, dur time.Duration, dialDur time.Duration, err error)
	Resolver() netResolver
}

func runCheck(ctx context.Context, chk ConnectivityCheck, chkr Checker, delay time.Duration) {
	select {
	case <-time.After(delay):
		// continue
	case <-ctx.Done():
		return
	}

	t := time.NewTicker(chk.Interval)
	defer t.Stop()
	for {
		log.Printf("Running check %s for %s/%s...", chk.Kind.String(), chk.Network, chk.Host)
		if err := doCheck(ctx, &chk, chkr); err != nil {
			checkFailures.WithLabelValues(chk.Network, chk.Host, chk.Service, chk.Kind.String()).Inc()
			log.Printf("Failed check %s for %s/%s (ignored): %v", chk.Kind.String(), chk.Network, chk.Host, err)
		}

		select {
		case <-t.C:
			// continue
		case <-ctx.Done():
			return
		}
	}
}

func doCheck(ctx context.Context, chk *ConnectivityCheck, chkr Checker) error {
	// We resolve before the checking code so we're sure we're not
	// measuring default resolver performance/availability.
	addrs, err := chkr.Resolver().LookupIP(ctx, chk.Network, chk.Host)
	if err != nil {
		return err
	}
	network := "ip6"
	if addrs[0].To4() != nil {
		network = "ip4"
	}
	host := addrs[0].String()
	var port string
	if chk.Service != "" {
		prt, err := chkr.Resolver().LookupPort(ctx, transportForNetwork(chk.Network, chk.Kind), chk.Service)
		if err != nil {
			return err
		}
		port = strconv.FormatInt(int64(prt), 10)
	}

	switch chk.Kind {
	case KindHostPing:
		st, err := chkr.CheckPing(ctx, network, host, false)
		if err != nil {
			return err
		}
		hostRTT.WithLabelValues(chk.Network, chk.Host).Set(float64(st.AvgRtt) / float64(time.Second))

	case KindHostFloodPing:
		st, err := chkr.CheckPing(ctx, network, host, true)
		if err != nil {
			return err
		}
		hostPacketLoss.WithLabelValues(chk.Network, chk.Host).Set(st.PacketLoss)
		hostRTT.WithLabelValues(chk.Network, chk.Host).Set(float64(st.AvgRtt) / float64(time.Second))

	case KindConnect:
		dur, err := chkr.CheckConnect(ctx, network, host, port)
		if err != nil {
			return err
		}
		serviceLatency.WithLabelValues(chk.Network, chk.Host, chk.Service, chk.Kind.String()).Set(float64(dur) / float64(time.Second))

	case KindTransfer:
		nbytes, dur, connDur, err := chkr.CheckTransfer(ctx, network, host, port)
		if err != nil {
			return err
		}
		serviceLatency.WithLabelValues(chk.Network, chk.Host, chk.Service, chk.Kind.String()).Set(float64(connDur) / float64(time.Second))
		serviceThroughput.WithLabelValues(chk.Network, chk.Host, chk.Service, chk.Kind.String()).Set(float64(nbytes) / (float64(dur) / float64(time.Second)))

	default:
		return fmt.Errorf("unknown check kind: %v", chk.Kind)
	}

	return nil
}

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

func (checker) Resolver() netResolver {
	return defaultResolver
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

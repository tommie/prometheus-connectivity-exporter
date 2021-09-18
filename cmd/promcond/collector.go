package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/go-ping/ping"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	collectionFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "connectivity",
		Name:      "collection_failures",
		Help:      "Collection failures while connecting.",
	}, []string{"af", "host", "service", "kind"})
)

func init() {
	prometheus.MustRegister(collectionFailures)
}

// A conCollector uses various network protocols to export
// connectivity metrics.
type conCollector struct {
	// ctx is the context to use while collecting. Only set it if the
	// collector is created per request. It would be better if Collect
	// took a `Context` directly.
	ctx    context.Context
	checks []ConnectivityCheck
	chkr   Checker

	hostPacketLossDesc    *prometheus.Desc
	hostRTTDesc           *prometheus.Desc
	serviceLatencyDesc    *prometheus.Desc
	serviceThroughputDesc *prometheus.Desc
}

// newConnectivityCollector creates a new collector.
func newConnectivityCollector(ctx context.Context, checks []ConnectivityCheck) *conCollector {
	return &conCollector{
		ctx:    ctx,
		checks: checks,
		chkr:   checker{},

		// In this case, reporting the ratio itself is probably
		// right. I can't see that we'd want this weighted by number
		// of packages rather than by host.
		hostPacketLossDesc:    prometheus.NewDesc("connectivity_host_packet_loss", "Packet loss between instance and remote host.", []string{"af", "host"}, nil),
		hostRTTDesc:           prometheus.NewDesc("connectivity_host_rtt", "RTT between instance and remote host.", []string{"af", "host"}, nil),
		serviceLatencyDesc:    prometheus.NewDesc("connectivity_service_latency", "Latency between the instance and a remote service.", []string{"af", "host", "service", "kind"}, nil),
		serviceThroughputDesc: prometheus.NewDesc("connectivity_service_throughput", "Whether the instance can use a remote service.", []string{"af", "host", "service", "kind"}, nil),
	}
}

// Describe implements prometheus.Collector.
func (c *conCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.hostPacketLossDesc
	ch <- c.hostRTTDesc
	ch <- c.serviceLatencyDesc
	ch <- c.serviceThroughputDesc
}

// Collector implements prometheus.Collector.
func (c *conCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	for _, chk := range c.checks {
		if err := c.doCheck(ctx, ch, defaultResolver, &chk); err != nil {
			collectionFailures.WithLabelValues(chk.Network, chk.Host, chk.Service, chk.Kind.String()).Inc()
			log.Printf("Failed check %s for %s/%s (ignored): %v", chk.Kind.String(), chk.Network, chk.Host, err)
		}
	}
}

func (c *conCollector) doCheck(ctx context.Context, ch chan<- prometheus.Metric, res netResolver, chk *ConnectivityCheck) error {
	// We resolve before the checking code so we're sure we're not
	// measuring default resolver performance/availability.
	addrs, err := res.LookupIP(ctx, chk.Network, chk.Host)
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
		prt, err := res.LookupPort(ctx, transportForNetwork(chk.Network, chk.Kind), chk.Service)
		if err != nil {
			return err
		}
		port = strconv.FormatInt(int64(prt), 10)
	}

	switch chk.Kind {
	case KindHostPing:
		st, err := c.chkr.CheckPing(ctx, network, host, false)
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(c.hostRTTDesc, prometheus.GaugeValue, float64(st.AvgRtt)/float64(time.Second), chk.Network, chk.Host)

	case KindHostFloodPing:
		st, err := c.chkr.CheckPing(ctx, network, host, true)
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(c.hostPacketLossDesc, prometheus.GaugeValue, st.PacketLoss, chk.Network, chk.Host)
		ch <- prometheus.MustNewConstMetric(c.hostRTTDesc, prometheus.GaugeValue, float64(st.AvgRtt)/float64(time.Second), chk.Network, chk.Host)

	case KindConnect:
		dur, err := c.chkr.CheckConnect(ctx, network, host, port)
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(c.serviceLatencyDesc, prometheus.GaugeValue, float64(dur)/float64(time.Second), chk.Network, chk.Host, chk.Service, chk.Kind.String())

	case KindTransfer:
		nbytes, dur, connDur, err := c.chkr.CheckTransfer(ctx, network, host, port)
		if err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(c.serviceLatencyDesc, prometheus.GaugeValue, float64(connDur)/float64(time.Second), chk.Network, chk.Host, chk.Service, chk.Kind.String())
		ch <- prometheus.MustNewConstMetric(c.serviceThroughputDesc, prometheus.GaugeValue, float64(nbytes)/(float64(dur)/float64(time.Second)), chk.Network, chk.Host, chk.Service, chk.Kind.String())

	default:
		return fmt.Errorf("unknown check kind: %v", chk.Kind)
	}

	return nil
}

// ConnectivityCheck encapsulates a single check against a host or service on a host.
type ConnectivityCheck struct {
	Kind    ConnectivityCheckKind
	Network string
	Host    string
	Service string
}

// A Checker is used by the conCollector to do the actual checking.
type Checker interface {
	CheckPing(ctx context.Context, network, host string, flood bool) (*ping.Statistics, error)
	CheckConnect(ctx context.Context, network, host, service string) (time.Duration, error)
	CheckTransfer(ctx context.Context, network, host, service string) (nbytes int, dur time.Duration, dialDur time.Duration, err error)
}

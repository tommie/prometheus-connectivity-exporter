package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-ping/ping"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestConnectivityCollector(t *testing.T) {
	ctx := context.Background()

	c := newConnectivityCollector(ctx, []ConnectivityCheck{
		{Kind: KindHostPing, Network: "ip", Host: "localhost"},
		{Kind: KindHostFloodPing, Network: "ip4", Host: "localhost"},
		{Kind: KindConnect, Network: "ip", Host: "localhost", Service: "42"},
		{Kind: KindTransfer, Network: "ip4", Host: "localhost", Service: "42"},
	})
	var chkr fakeChecker
	c.chkr = chkr
	want := `
# HELP connectivity_host_packet_loss Packet loss between instance and remote host.
# TYPE connectivity_host_packet_loss gauge
connectivity_host_packet_loss{af="ip4",host="localhost"} 0.5

# HELP connectivity_host_rtt RTT between instance and remote host.
# TYPE connectivity_host_rtt gauge
connectivity_host_rtt{af="ip",host="localhost"} 1
connectivity_host_rtt{af="ip4",host="localhost"} 1

# HELP connectivity_service_latency Latency between the instance and a remote service.
# TYPE connectivity_service_latency gauge
connectivity_service_latency{af="ip",host="localhost",kind="connect",service="42"} 2
connectivity_service_latency{af="ip4",host="localhost",kind="transfer",service="42"} 3

# HELP connectivity_service_throughput Whether the instance can use a remote service.
# TYPE connectivity_service_throughput gauge
connectivity_service_throughput{af="ip4",host="localhost",kind="transfer",service="42"} 256
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(want)); err != nil {
		t.Errorf("CollectAndCompare: %v", err)
	}
}

type fakeChecker struct{}

func (fakeChecker) CheckPing(ctx context.Context, network, host string, flood bool) (*ping.Statistics, error) {
	return &ping.Statistics{AvgRtt: 1 * time.Second, PacketLoss: 0.5}, nil
}
func (fakeChecker) CheckConnect(ctx context.Context, network, host, service string) (time.Duration, error) {
	return 2 * time.Second, nil
}
func (fakeChecker) CheckTransfer(ctx context.Context, network, host, service string) (nbytes int, dur time.Duration, dialDur time.Duration, err error) {
	return 1024, 4 * time.Second, 3 * time.Second, nil
}

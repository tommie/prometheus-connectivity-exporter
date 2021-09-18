# Prometheus Connectivity Exporter

[![CircleCI](https://circleci.com/gh/tommie/prometheus-connectivity-exporter/tree/main.svg?style=svg)](https://circleci.com/gh/tommie/prometheus-connectivity-exporter/tree/main)

This is a [Prometheus](https://prometheus.io/) exporter that probes
connectivity to other network hosts. This can be useful to diagnose
regressions in client machines using (unreliable) ISPs. The use on
servers in a controlled network is probably limited.

## Usage

Run it with Docker or Docker Compose. The default port is 9232. The
`/metrics` endpoint contains information about the exporter itself,
while `/probe` is the [multi-target
exporter](https://prometheus.io/docs/guides/multi-target-exporter/)
endpoint.

### URL Parameters

* `kind`: the kind of check to perform. See the following sections.
* `af`: the address family. One of `ip`, `ip4` and `ip6`.
* `target`: the host address. A hostname or IP-address.
* `service`: some check kinds use a specific service/port. Either a
  symbolic service name, like `ssh`, or a number.

### Check Kinds

* `ping`: sends a few UDP echo requests and measures RTT.
* `floodping`: sends many UDP echo requests and measures both RTT
  and packet loss.
* `connect`: do a TCP connect and measure latency.
* `transfer`: do a TCP connect, transfer some data and report
  latency and throughput. This requires the target to run a
  [chargen2p server](https://pkg.go.dev/github.com/tommie/chargen2p).

### Target Names

Targets are hostnames or IP-addresses. The special name
`default-gateway.internal` makes the exporter look up (one of) the
host's default gateway(s).

## Metrics

The following metrics are exported as part of a `/probe`, depending
on the kind of check being performed:

* `connectivity_host_packet_loss{af,host}`: packet loss as a
  fraction between zero and one.
* `connectivity_host_rtt{af,host}`: round-trip-time, in seconds.
* `connectivity_service_latency{af,host,service,kind}`: latency
  estimation for talking to the given service, in seconds.
* `connectivity_service_throughput{af,host,service,kind}`:
  throughput estimation for talking to the given service, in bytes
  per second.

## License

This project is licensed under the [MIT license](LICENSE).

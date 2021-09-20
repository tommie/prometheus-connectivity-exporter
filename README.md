# Prometheus Connectivity Exporter

[![CircleCI](https://circleci.com/gh/tommie/prometheus-connectivity-exporter/tree/main.svg?style=svg)](https://circleci.com/gh/tommie/prometheus-connectivity-exporter/tree/main)

This is a [Prometheus](https://prometheus.io/) exporter that probes
connectivity to other network hosts. This can be useful to diagnose
regressions in client machines using (unreliable) ISPs. The use on
servers in a controlled network is probably limited.

## Usage

Run it with Docker or Docker Compose. The default port is 9232. The
`/metrics` endpoint contains information about the checks and the
exporter itself.

### Command Line Flags

The most important flag is `-check`, which adds a new check to the
list. Each flag value is on the format `key=value[,key=value...]`.

The keys are

* `kind`: the kind of check to perform. See the following sections.
* `af`: the address family. One of `ip`, `ip4` and `ip6`. The
  default is `ip`.
* `target`: the host address. A hostname or IP-address.
* `service`: some check kinds use a specific service/port. Either a
  symbolic service name, like `ssh`, or a number.
* `interval`: a time duration value like `1m10s`. This is how often
  the check should run. If a check takes longer than the interval,
  checks will be skipped, but the pace is kept.

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

## Prior Work

* [`blackbox_exporter`](https://github.com/prometheus/blackbox_exporter)
  is a connectivity tester for the data center. It uses [multi-target
  exporter](https://prometheus.io/docs/guides/multi-target-exporter/). The
  main difference is our support for `default-gateway.internal`, and
  that the connectivity exporter cares about which host is able to
  connect to which target. The Blackbox exporter mostly just cares
  about the target.
* The various [`ping`](https://github.com/czerwonk/ping_exporter)
  exporters.

## License

This project is licensed under the [MIT license](LICENSE).

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// startCollectorServer reads global flags and starts the HTTP
// server. Callers should run the returned cleanup function once the
// server is stopped.
func startCollectorServer(ctx context.Context, httpAddr string, chkr Checker) (net.Listener, *http.Server, func(), error) {
	http.Handle("/probe", probeHandler{chkr: chkr})
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/metrics", http.StatusFound)
	})

	l, err := net.Listen("tcp", httpAddr)
	if err != nil {
		return nil, nil, nil, err
	}

	s := &http.Server{Addr: l.Addr().String()}

	cctx, cancel := context.WithCancel(ctx)
	stopHTTPServerOnSignal(cctx, s, os.Interrupt, syscall.SIGTERM)

	return l, s, cancel, nil
}

type probeHandler struct {
	chkr Checker
}

func (h probeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cc, err := connectivityCheckFromURL(r.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	coll := newConnectivityCollector(r.Context(), []ConnectivityCheck{cc})
	if h.chkr != nil {
		coll.chkr = h.chkr
	}
	reg := prometheus.NewRegistry()
	if err := reg.Register(coll); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// InstrumentMetricHandler can handle its metrics having already
	// been registered.
	var ph http.Handler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	ph = promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer, ph)
	ph.ServeHTTP(w, r)
}

func connectivityCheckFromURL(u *url.URL) (ConnectivityCheck, error) {
	kind, err := parseConnectivityCheckKind(u.Query().Get("kind"))
	if err != nil {
		return ConnectivityCheck{}, err
	}

	cc := ConnectivityCheck{
		Kind:    kind,
		Network: u.Query().Get("af"),
		Host:    u.Query().Get("target"),
		Service: u.Query().Get("service"),
	}
	if cc.Network == "" {
		cc.Network = "ip"
	}
	if cc.Host == "" {
		return ConnectivityCheck{}, fmt.Errorf("missing target parameter")
	}
	if cc.Service == "" {
		switch cc.Kind {
		case KindHostPing, KindHostFloodPing:
			// Don't need service.
		default:
			return ConnectivityCheck{}, fmt.Errorf("missing service parameter")
		}
	}

	return cc, nil
}

// stopHTTPServerOnSignal listens for OS signals, and returns. On
// signal, it runs s.Shutdown. If another signal is raised, s.Close is
// called. Honors context cancellation.
func stopHTTPServerOnSignal(ctx context.Context, s shutdownerCloser, sigs ...os.Signal) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sigs...)

	go func() {
		defer close(ch)
		defer signal.Stop(ch)

		select {
		case <-ch:
			// continue
		case <-ctx.Done():
			return
		}
		if err := s.Shutdown(ctx); err != nil {
			log.Printf("Shutdown failed: %v", err)
		}

		select {
		case <-ch:
			// continue
		case <-ctx.Done():
			return
		}
		if err := s.Close(); err != nil {
			log.Printf("Close failed: %v", err)
		}
	}()
}

type shutdownerCloser interface {
	Shutdown(context.Context) error
	Close() error
}

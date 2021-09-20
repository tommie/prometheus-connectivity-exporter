// Command promcond is a network connectivity exporter for Prometheus.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

var (
	httpAddr      = flag.String("http-addr", "localhost:0", "TCP-address to listen for HTTP connections on.")
	standaloneLog = flag.Bool("standalone-log", true, "Log to stderr, with time prefix.")
	checks        = checkSliceFlag("check", "Add a check to perform, in the format 'kind=X,af=Y,host=Z,service=W,interval=T'.")
)

func main() {
	flag.Parse()

	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// run starts everything and waits for a signal to terminate.
func run(ctx context.Context) error {
	if !*standaloneLog {
		log.SetFlags(0)
		log.SetOutput(os.Stdout)
	}

	if len(*checks) == 0 {
		return fmt.Errorf("no -check flags provided")
	}
	startChecks(ctx, *checks, checker{})

	l, s, cleanup, err := startMetricsServer(ctx, *httpAddr)
	if err != nil {
		return err
	}
	defer cleanup()

	log.Printf("Listening for HTTP connections on %q...", s.Addr)
	if err := s.Serve(l); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

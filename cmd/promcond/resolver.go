package main

import (
	"context"
	"net"

	"github.com/jackpal/gateway"
)

// defautResolver is the default resolver for the collector.
var defaultResolver = &keywordResolver{
	netResolver:     net.DefaultResolver,
	discoverGateway: gateway.DiscoverGateway,
}

// A keywordResolver intercepts some lookups to resolve magic
// keywords.
//
//  default-gateway.internal - Resolves to one of the default gateways of the host.
type keywordResolver struct {
	netResolver
	discoverGateway func() (net.IP, error)
}

// A netResolver is our interest in a net.Resolver.
type netResolver interface {
	LookupIP(context.Context, string, string) ([]net.IP, error)
	LookupPort(context.Context, string, string) (int, error)
}

func (r *keywordResolver) LookupIP(ctx context.Context, network, host string) ([]net.IP, error) {
	switch host {
	case "default-gateway.internal":
		ip, err := r.discoverGateway()
		if err != nil {
			return nil, err
		}
		host = ip.String()
	}

	return r.netResolver.LookupIP(ctx, network, host)
}

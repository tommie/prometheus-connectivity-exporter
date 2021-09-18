package main

import (
	"context"
	"net"
	"reflect"
	"testing"
)

func TestKeywordResolver(t *testing.T) {
	ctx := context.Background()

	t.Run("LookupIP_net", func(t *testing.T) {
		var fnr fakeNetResolver
		res := &keywordResolver{
			netResolver: &fnr,
		}

		got, err := res.LookupIP(ctx, "anetwork", "ahost")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if want := []lookupIPCall{{"anetwork", "ahost"}}; !reflect.DeepEqual(fnr.LookupIPCalls, want) {
			t.Errorf("LookupIPCalls: got %+v, want %+v", fnr.LookupIPCalls, want)
		}
		if want := []net.IP{net.IPv4bcast}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("LookupIP_gw", func(t *testing.T) {
		var fnr fakeNetResolver
		res := &keywordResolver{
			netResolver: &fnr,
			discoverGateway: func() (net.IP, error) {
				return net.IPv4allsys, nil
			},
		}

		got, err := res.LookupIP(ctx, "anetwork", "default-gateway.internal")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if want := []lookupIPCall{{"anetwork", net.IPv4allsys.String()}}; !reflect.DeepEqual(fnr.LookupIPCalls, want) {
			t.Errorf("LookupIPCalls: got %+v, want %+v", fnr.LookupIPCalls, want)
		}
		if want := []net.IP{net.IPv4bcast}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, don't want %+v", got, want)
		}
	})
}

type fakeNetResolver struct {
	LookupIPCalls []lookupIPCall
}

type lookupIPCall struct {
	Network string
	Host    string
}

func (r *fakeNetResolver) LookupIP(_ context.Context, network, host string) ([]net.IP, error) {
	r.LookupIPCalls = append(r.LookupIPCalls, lookupIPCall{network, host})
	return []net.IP{net.IPv4bcast}, nil
}

func (r *fakeNetResolver) LookupPort(_ context.Context, network, service string) (int, error) {
	return 42, nil
}

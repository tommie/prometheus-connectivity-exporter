package main

import (
	"flag"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCheckSliceFlagSet(t *testing.T) {
	tsts := []struct {
		S       string
		Want    ConnectivityCheck
		WantErr string
	}{
		{"", ConnectivityCheck{}, `got ""`},
		{"kind=ping", ConnectivityCheck{}, `missing host`},
		{"kind=ping,host=a", ConnectivityCheck{}, `missing interval`},
		{"kind=ping,host=a,interval=1m", ConnectivityCheck{Kind: KindHostPing, Network: "ip", Host: "a", Interval: 1 * time.Minute}, ""},
		{"kind=connect,host=a,interval=1m", ConnectivityCheck{Kind: KindConnect, Network: "ip", Host: "a", Interval: 1 * time.Minute}, "missing service"},
		{"kind=connect,host=a,service=b,interval=1m", ConnectivityCheck{Kind: KindConnect, Network: "ip", Host: "a", Service: "b", Interval: 1 * time.Minute}, ""},
	}
	for _, tst := range tsts {
		t.Run(tst.S, func(t *testing.T) {
			var set flag.FlagSet
			got := checkSliceFlagSet(&set, "c", "help")

			if err := set.Parse([]string{"-c", tst.S}); tst.WantErr == "" && err != nil {
				t.Fatalf("Parse failed: %v", err)
			} else if tst.WantErr != "" {
				if !strings.Contains(err.Error(), tst.WantErr) {
					t.Fatalf("Parse err: got %v, want containing %q", err, tst.WantErr)
				}
				return
			}

			if !reflect.DeepEqual(*got, []ConnectivityCheck{tst.Want}) {
				t.Errorf("Parse: got %+v, want %+v", got, tst.Want)
			}
		})
	}

	t.Run("append", func(t *testing.T) {
		var set flag.FlagSet
		got := checkSliceFlagSet(&set, "c", "help")

		if err := set.Parse([]string{"-c", "kind=ping,host=a,interval=1m", "-c", "kind=ping,host=b,interval=1m"}); err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		want := []ConnectivityCheck{
			ConnectivityCheck{Kind: KindHostPing, Network: "ip", Host: "a", Interval: 1 * time.Minute},
			ConnectivityCheck{Kind: KindHostPing, Network: "ip", Host: "b", Interval: 1 * time.Minute},
		}
		if !reflect.DeepEqual(*got, want) {
			t.Errorf("Parse: got %+v, want %+v", got, want)
		}
	})
}

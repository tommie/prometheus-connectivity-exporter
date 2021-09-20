package main

import (
	"flag"
	"fmt"
	"strings"
	"time"
)

func checkSliceFlag(name, usage string) *[]ConnectivityCheck {
	return checkSliceFlagSet(flag.CommandLine, name, usage)
}

func checkSliceFlagSet(set *flag.FlagSet, name, usage string) *[]ConnectivityCheck {
	var ccs []ConnectivityCheck

	set.Func(name, usage, func(s string) error {
		cc := ConnectivityCheck{
			Network: "ip",
		}

		ss := strings.Split(s, ",")
		for _, kv := range ss {
			kvs := strings.SplitN(kv, "=", 2)
			if len(kvs) == 1 {
				return fmt.Errorf("expected key=value[,...] in check flag, got %q", s)
			}
			switch kvs[0] {
			case "kind":
				var err error
				cc.Kind, err = parseConnectivityCheckKind(kvs[1])
				if err != nil {
					return err
				}
			case "af":
				cc.Network = kvs[1]
			case "host":
				cc.Host = kvs[1]
			case "service":
				cc.Service = kvs[1]
			case "interval":
				var err error
				cc.Interval, err = time.ParseDuration(kvs[1])
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("unexpected key in check flag: %v", kvs[0])
			}
		}
		if cc.Host == "" {
			return fmt.Errorf("missing host parameter: %s", s)
		}
		if cc.Service == "" {
			switch cc.Kind {
			case KindHostPing, KindHostFloodPing:
				// Don't need service.
			default:
				return fmt.Errorf("missing service parameter: %s", s)
			}
		}
		if cc.Interval == 0 {
			return fmt.Errorf("missing interval parameter: %s", s)
		}
		ccs = append(ccs, cc)
		return nil
	})

	return &ccs
}

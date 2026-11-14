package collector

import (
	"context"
	"github.com/shellghostt/envdoctor/internal/system"
	"sort"
)

type PortCollector struct{ Requested []int }

func (PortCollector) Name() string { return "network" }
func (c PortCollector) Collect(ctx context.Context) (Part, error) {
	if len(c.Requested) == 0 {
		return Part{Ports: []Port{}}, nil
	}
	sockets, err := system.ListeningPorts(ctx)
	if err != nil {
		return Part{}, err
	}
	out := []Port{}
	seen := map[[2]int]bool{}
	for _, s := range sockets {
		for _, wanted := range c.Requested {
			key := [2]int{s.Port, s.PID}
			if s.Port == wanted && !seen[key] {
				seen[key] = true
				out = append(out, Port{Port: s.Port, Protocol: "tcp", State: "listen", PID: s.PID, Process: s.Process})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Port == out[j].Port {
			return out[i].PID < out[j].PID
		}
		return out[i].Port < out[j].Port
	})
	return Part{Ports: out}, nil
}

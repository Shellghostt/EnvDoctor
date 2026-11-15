package system

import (
	"context"
	"errors"
	"net"
	"strconv"
)

// ProbeProxy makes one TCP connection to one configured endpoint. It sends no
// application data. The caller supplies the opt-in and an overall deadline.
func ProbeProxy(ctx context.Context, host string, port int) (string, error) {
	ip := net.ParseIP(host)
	if ip == nil {
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return "unknown", errors.New("proxy DNS lookup failed or timed out")
		}
		if len(addresses) == 0 {
			return "unknown", errors.New("proxy DNS lookup returned no addresses")
		}
		ip = addresses[0].IP
	}
	if err := ctx.Err(); err != nil {
		return "unknown", err
	}
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), strconv.Itoa(port)))
	if err != nil {
		if ctx.Err() != nil {
			return "unknown", ctx.Err()
		}
		if proxyUnreachableError(err) {
			return "unreachable", nil
		}
		return "unknown", errors.New("proxy connection failed without conclusive reachability evidence")
	}
	if err := conn.Close(); err != nil {
		return "unknown", errors.New("proxy connection could not be closed cleanly")
	}
	return "reachable", nil
}

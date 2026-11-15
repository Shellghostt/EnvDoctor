package collector

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shellghostt/envdoctor/internal/system"
)

const ProxyProbeTimeout = time.Second

type ProxyCollector struct {
	ProbeNetwork bool
	Getenv       func(string) string
	Probe        func(context.Context, string, int) (string, error)
}

func (ProxyCollector) Name() string { return "proxy" }

func (c ProxyCollector) Collect(ctx context.Context) (Part, error) {
	getenv, probe := c.Getenv, c.Probe
	if getenv == nil {
		getenv = system.Getenv
	}
	if probe == nil {
		probe = system.ProbeProxy
	}
	part := Part{Proxies: []Proxy{}, NoProxyConfigured: getenv("NO_PROXY") != "" || getenv("no_proxy") != ""}
	probeCtx, cancel := context.WithTimeout(ctx, ProxyProbeTimeout)
	defer cancel()
	type result struct {
		status string
		err    error
	}
	results := map[string]result{}
	for _, variable := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"} {
		if err := ctx.Err(); err != nil {
			return Part{}, err
		}
		value := getenv(variable)
		if value == "" {
			value = getenv(strings.ToLower(variable))
		}
		if value == "" {
			continue
		}
		p, err := proxyEndpoint(variable, value)
		if err != nil {
			part.Proxies = append(part.Proxies, Proxy{Variable: variable, URL: "[invalid]", Probe: "invalid"})
			part.Errors = append(part.Errors, CollectorError{Collector: "proxy", Message: variable + " has an invalid or unsupported proxy URL"})
			continue
		}
		if c.ProbeNetwork {
			key := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
			r, ok := results[key]
			if !ok {
				if probeCtx.Err() != nil {
					r = result{"unknown", probeCtx.Err()}
				} else {
					r.status, r.err = probe(probeCtx, p.Host, p.Port)
					if probeCtx.Err() != nil {
						r = result{"unknown", probeCtx.Err()}
					}
					if r.status != "reachable" && r.status != "unreachable" && r.status != "unknown" {
						r = result{"unknown", fmt.Errorf("proxy probe returned unsupported evidence")}
					}
				}
				results[key] = r
			}
			p.Probe = r.status
			if r.err != nil || r.status == "unknown" {
				p.Probe = "unknown"
				part.Errors = append(part.Errors, CollectorError{Collector: "proxy", Message: variable + " reachability is unknown (probe failed, timed out or was canceled)"})
			}
		}
		part.Proxies = append(part.Proxies, p)
	}
	return part, nil
}

func proxyEndpoint(variable, value string) (Proxy, error) {
	if len(value) > 8192 {
		return Proxy{}, fmt.Errorf("proxy URL too long")
	}
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || u.Opaque != "" {
		return Proxy{}, fmt.Errorf("invalid proxy URL")
	}
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return Proxy{}, fmt.Errorf("invalid or unsupported proxy host")
	}
	if !strings.Contains(host, ":") {
		for _, ch := range host {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789-._", ch) {
				return Proxy{}, fmt.Errorf("unsupported proxy hostname")
			}
		}
	}
	port := 0
	switch strings.ToLower(u.Scheme) {
	case "http":
		port = 80
	case "https":
		port = 443
	case "socks5", "socks5h":
		port = 1080
	default:
		return Proxy{}, fmt.Errorf("unsupported proxy scheme")
	}
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return Proxy{}, fmt.Errorf("invalid proxy port")
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return Proxy{}, fmt.Errorf("empty proxy port")
	}
	return Proxy{Variable: variable, URL: strings.ToLower(u.Scheme) + "://" + net.JoinHostPort(host, strconv.Itoa(port)), Host: host, Port: port, Probe: "untested"}, nil
}

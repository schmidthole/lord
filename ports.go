package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// validatePortMapping accepts explicit, single-port Docker publications only.
// Requiring both ports avoids accidental ephemeral bindings. An IP, if given,
// must be literal; Docker does not resolve hostnames in published bindings.
func validatePortMapping(mapping string) error {
	binding, protocol, hasProtocol := strings.Cut(mapping, "/")
	if hasProtocol && protocol != "tcp" && protocol != "udp" && protocol != "sctp" {
		return fmt.Errorf("protocol must be tcp, udp, or sctp")
	}
	separator := strings.LastIndex(binding, ":")
	if separator < 0 {
		return fmt.Errorf("expected [host IP:]host port:container port[/protocol]")
	}
	hostPort, containerPort := binding[:separator], binding[separator+1:]
	if strings.Contains(hostPort, ":") {
		host, port, err := net.SplitHostPort(hostPort)
		if err != nil || net.ParseIP(host) == nil {
			return fmt.Errorf("host must be an IPv4 or bracketed IPv6 address")
		}
		hostPort = port
	}
	for _, port := range []string{hostPort, containerPort} {
		if port == "" || strings.Trim(port, "0123456789") != "" {
			return fmt.Errorf("ports must be decimal integers from 1 to 65535")
		}
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil || number == 0 {
			return fmt.Errorf("ports must be decimal integers from 1 to 65535")
		}
	}
	return nil
}

// Quote each mapping as one argument to the remote POSIX shell. Validation is
// performed at config load, but quoting also protects programmatic callers.
func shellQuotePort(mapping string) string {
	return "'" + strings.ReplaceAll(mapping, "'", "'\"'\"'") + "'"
}

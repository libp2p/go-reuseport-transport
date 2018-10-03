package reusetransport

import (
	"context"
	"net"

	reuseport "github.com/libp2p/go-reuseport"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

type dialer interface {
	Dial(network, addr string) (net.Conn, error)
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// Dial dials the given multiaddr, reusing ports we're currently listening on if
// possible.
//
// Dial attempts to be smart about choosing the source port. For example, If
// we're dialing a loopback address and we're listening on one or more loopback
// ports, Dial will randomly choose one of the loopback ports and addresses and
// reuse it.
func (t *Transport) Dial(raddr ma.Multiaddr) (manet.Conn, error) {
	return t.DialContext(context.Background(), raddr)
}

// DialContext is like Dial but takes a context.
func (t *Transport) DialContext(ctx context.Context, raddr ma.Multiaddr) (manet.Conn, error) {
	network, addr, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}
	var d dialer
	switch network {
	case "tcp4":
		d = t.v4.getTcpDialer(network)
	case "udp4":
		d = t.v4.getUdpDialer(network)
	case "tcp6":
		d = t.v6.getTcpDialer(network)
	case "udp6":
		d = t.v6.getUdpDialer(network)
	default:
		return nil, ErrWrongDialProto
	}
	conn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	maconn, err := manet.WrapNetConn(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return maconn, nil
}

func (n *network) getTcpDialer(network string) dialer {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.tcpDialer != nil {
		return n.tcpDialer
	}
	n.tcpDialer = n.makeDialer(network)
	return n.tcpDialer
}

func (n *network) getUdpDialer(network string) dialer {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.udpDialer != nil {
		return n.udpDialer
	}
	n.udpDialer = n.makeDialer(network)
	return n.udpDialer
}

func tcpAddresses(listeners map[*listener]struct{}) []net.Addr {
	result := make([]net.Addr, 0, len(listeners))
	for l := range listeners {
		result = append(result, l.Addr())
	}
	return result
}

func udpAddresses(listeners map[*udpListener]struct{}) []net.Addr {
	result := make([]net.Addr, 0, len(listeners))
	for l := range listeners {
		result = append(result, l.Connection().LocalAddr()) // TODO make udpListener's interface comparable to listener
	}
	return result
}

func ipOf(addr net.Addr) net.IP {
	if a, ok := addr.(*net.TCPAddr); ok {
		return a.IP
	}
	if a, ok := addr.(*net.UDPAddr); ok {
		return a.IP
	}
	panic("only support tcp and udp address")
}

func portOf(addr net.Addr) int {
	if a, ok := addr.(*net.TCPAddr); ok {
		return a.Port
	}
	if a, ok := addr.(*net.UDPAddr); ok {
		return a.Port
	}
	panic("only support tcp and udp address")
}

func (n *network) makeDialer(network string) dialer {
	if !reuseport.Available() {
		log.Debug("reuseport not available")
		return &net.Dialer{}
	}

	var unspec net.IP
	var listenAddrs []net.Addr
	switch network {
	case "tcp4":
		unspec = net.IPv4zero
		listenAddrs = tcpAddresses(n.tcpListeners)
	case "udp4":
		unspec = net.IPv4zero
		listenAddrs = udpAddresses(n.udpListeners)
	case "tcp6":
		unspec = net.IPv6unspecified
		listenAddrs = tcpAddresses(n.tcpListeners)
	case "udp6":
		unspec = net.IPv6unspecified
		listenAddrs = udpAddresses(n.udpListeners)
	default:
		panic("invalid network: must be either tcp4, tcp6, udp4 or udp6")
	}

	// How many ports are we listening on.
	var port = 0
	for _, l := range listenAddrs {
		newPort := portOf(l)
		switch {
		case newPort == 0: // Any port, ignore (really, we shouldn't get this case...).
		case port == 0: // Haven't selected a port yet, choose this one.
			port = newPort
		case newPort == port: // Same as the selected port, continue...
		default: // Multiple ports, use the multi dialer
			return newMultiDialer(unspec, listenAddrs, network)
		}
	}

	// None.
	if port == 0 {
		return &net.Dialer{}
	}

	// One. Always dial from the single port we're listening on.
	switch network {
	case "tcp4", "tcp6":
		laddr := &net.TCPAddr{
			IP:   unspec,
			Port: port,
		}
		return singleDialer{laddr}
	case "udp4", "udp6":
		laddr := &net.UDPAddr{
			IP:   unspec,
			Port: port,
		}
		return singleDialer{laddr}
	default:
		panic("invalid network: must be either tcp4, tcp6, udp4 or udp6")
	}
}

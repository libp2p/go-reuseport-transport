package tcpreuse

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

func (t *Transport) Dial(raddr ma.Multiaddr) (manet.Conn, error) {
	return t.DialContext(context.Background(), raddr)
}

func (t *Transport) DialContext(ctx context.Context, raddr ma.Multiaddr) (manet.Conn, error) {
	network, addr, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}
	var d dialer
	switch network {
	case "tcp4":
		d = t.v4.getDialer(network)
	case "tcp6":
		d = t.v6.getDialer(network)
	default:
		return nil, ErrWrongProto
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

func (n *network) getDialer(network string) dialer {
	n.mu.RLock()
	d := n.dialer
	n.mu.RUnlock()
	if d == nil {
		n.mu.Lock()
		defer n.mu.Unlock()

		if n.dialer == nil {
			n.dialer = n.makeDialer(network)
		}
		d = n.dialer
	}
	return d
}

func (n *network) makeDialer(network string) dialer {
	if !reuseport.Available() {
		log.Debug("reuseport not available")
		return &net.Dialer{}
	}

	var unspec net.IP
	switch network {
	case "tcp4":
		unspec = net.IPv4zero
	case "tcp6":
		unspec = net.IPv6unspecified
	default:
		panic("invalid network: must be either tcp4 or tcp6")
	}

	// How many ports are we listening on.
	var port = 0
	for l := range n.listeners {
		if port == 0 {
			port = l.Addr().(*net.TCPAddr).Port
		} else {
			// > 1
			return newMultiDialer(unspec, n.listeners)
		}
	}

	// None.
	if port == 0 {
		return &net.Dialer{}
	}

	// One. Always dial from the single port we're listening on.
	laddr := &net.TCPAddr{
		IP:   unspec,
		Port: port,
	}

	return (*singleDialer)(laddr)
}

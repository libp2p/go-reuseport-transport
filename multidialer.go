package tcpreuse

import (
	"context"
	"fmt"
	"math/rand"
	"net"
)

type multiDialer struct {
	loopback    []*net.TCPAddr
	unspecified []*net.TCPAddr
	global      *net.TCPAddr
}

func (d *multiDialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func randAddr(addrs []*net.TCPAddr) *net.TCPAddr {
	if len(addrs) > 0 {
		return addrs[rand.Intn(len(addrs))]
	}
	return nil
}

func (d *multiDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	tcpAddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return nil, err
	}

	ip := tcpAddr.IP
	source := d.global
	switch {
	case ip.IsLoopback():
		switch {
		case len(d.loopback) > 0:
			source = randAddr(d.loopback)
		case len(d.unspecified) > 0:
			source = randAddr(d.unspecified)
		}
	case ip.IsGlobalUnicast():
		switch {
		case len(d.unspecified) > 0:
			source = randAddr(d.unspecified)
		}
	default:
		return nil, fmt.Errorf("undialable IP: %s", tcpAddr.IP)
	}
	return reuseDial(ctx, source, network, addr)
}

func newMultiDialer(unspec net.IP, listeners map[*listener]struct{}) dialer {
	m := new(multiDialer)
	for l := range listeners {
		laddr := l.Addr().(*net.TCPAddr)
		switch {
		case laddr.IP.IsLoopback():
			m.loopback = append(m.loopback, laddr)
		case laddr.IP.IsGlobalUnicast():
			// Different global ports? Crap.
			//
			// The *proper* way to deal with this is to, e.g., use
			// netlink to figure out which source address we would
			// normally use to dial a destination address and then
			// pick one of the ports we're listening on on that
			// source address. However, this is a pain in the ass.
			//
			// Instead, we're just going to always dial from the
			// unspecified address with the first global port we
			// find.
			//
			// TODO: Port priority? Addr priority?
			if m.global != nil {
				m.global = &net.TCPAddr{
					IP:   unspec,
					Port: laddr.Port,
				}
			} else {
				log.Warning("listening on external interfaces on multiple ports, will dial from %d, not %s", m.global, laddr)
			}
		case laddr.IP.IsUnspecified():
			m.unspecified = append(m.unspecified, laddr)
		}
	}
	return m
}

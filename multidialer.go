package tcpreuse

import (
	"context"
	"math/rand"
	"net"

	"github.com/libp2p/go-netroute"
)

type multiDialer struct {
	listeners map[*listener]struct{}
	fallback  net.IP
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

	router, err := netroute.New()
	if err != nil {
		return nil, err
	}

	_, _, preferredSrcIP, err := router.Route(ip)
	if err != nil {
		return nil, err
	}

	unspecifiedCandidates := make([]*net.TCPAddr, 0)
	existingPort := 0
	for l := range d.listeners {
		optAddr := l.Addr().(*net.TCPAddr)
		if optAddr.IP.Equal(preferredSrcIP) {
			return reuseDial(ctx, optAddr, network, addr)
		}
		if optAddr.IP.IsUnspecified() {
			if optAddr.Network() == network {
				unspecifiedCandidates = append([]*net.TCPAddr{optAddr}, unspecifiedCandidates...)
			} else {
				unspecifiedCandidates = append(unspecifiedCandidates, optAddr)
			}
		}
		if optAddr.Network() == network {
			existingPort = optAddr.Port
		}
	}
	if len(unspecifiedCandidates) == 0 {
		unspecifiedCandidates = []*net.TCPAddr{&net.TCPAddr{IP: d.fallback, Port: existingPort, Zone: ""}}
	}

	return reuseDial(ctx, unspecifiedCandidates[0], network, addr)
}

func newMultiDialer(unspec net.IP, listeners map[*listener]struct{}) (m dialer) {
	m = &multiDialer{
		listeners: listeners,
		fallback:  unspec,
	}
	return
}

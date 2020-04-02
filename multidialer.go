package tcpreuse

import (
	"context"
	"fmt"
	"math/rand"
	"net"

	"github.com/libp2p/go-netroute"
)

type multiDialer struct {
	listeningAddresses []*net.TCPAddr
	fallback           net.IP
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

// DialContext dials a target addr.
// Dialing preference is
// * If there is a listener on the local interface the OS expects to use to route towards addr, use that.
// * If there is a listener on a loopback address, addr is loopback, use that.
// * If there is a listener on an undefined address (0.0.0.0 or ::), use that.
// * Use the fallback IP specified during construction, with a port that's already being listened on, if one exists.
func (d *multiDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	tcpAddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return nil, err
	}
	ip := tcpAddr.IP
	if !ip.IsLoopback() && !ip.IsGlobalUnicast() {
		return nil, fmt.Errorf("undialable IP: %s", ip)
	}

	router, err := netroute.New()
	if err != nil {
		return nil, err
	}

	_, _, preferredSrcIP, err := router.Route(ip)
	if err != nil {
		return nil, err
	}

	loopbackCandidates := make([]*net.TCPAddr, 0)
	unspecifiedCandidates := make([]*net.TCPAddr, 0)
	existingPort := 0
	for _, optAddr := range d.listeningAddresses {
		if optAddr.IP.Equal(preferredSrcIP) {
			return reuseDial(ctx, optAddr, network, addr)
		}
		if optAddr.IP.IsLoopback() {
			loopbackCandidates = append(loopbackCandidates, optAddr)
		} else if optAddr.IP.IsGlobalUnicast() && existingPort == 0 {
			existingPort = optAddr.Port
		} else if optAddr.IP.IsUnspecified() {
			unspecifiedCandidates = append(unspecifiedCandidates, optAddr)
		}
	}
	if ip.IsLoopback() && len(loopbackCandidates) > 0 {
		return reuseDial(ctx, randAddr(loopbackCandidates), network, addr)
	}
	if len(unspecifiedCandidates) == 0 {
		unspecifiedCandidates = []*net.TCPAddr{&net.TCPAddr{IP: d.fallback, Port: existingPort, Zone: ""}}
	}

	return reuseDial(ctx, randAddr(unspecifiedCandidates), network, addr)
}

func newMultiDialer(unspec net.IP, listeners map[*listener]struct{}) (m dialer) {
	addrs := make([]*net.TCPAddr, 0)
	for l := range listeners {
		addrs = append(addrs, l.Addr().(*net.TCPAddr))
	}
	m = &multiDialer{
		listeningAddresses: addrs,
		fallback:           unspec,
	}
	return
}

package reusetransport

import (
	"net"

	reuseport "github.com/libp2p/go-reuseport"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

type listener struct {
	manet.Listener
	network *network
}

type udpListener struct {
	manet.PacketConn
	network *network
}

func (l *listener) Close() error {
	l.network.mu.Lock()
	delete(l.network.tcpListeners, l)
	l.network.tcpDialer = nil
	l.network.mu.Unlock()
	return l.Listener.Close()
}

func (l *udpListener) Close() error {
	l.network.mu.Lock()
	delete(l.network.udpListeners, l)
	l.network.udpDialer = nil
	l.network.mu.Unlock()
	return l.PacketConn.Close()
}

// Listen listens on the given multiaddr.
//
// If reuseport is supported, it will be enabled for this listener and future
// dials from this transport may reuse the port.
//
// Note: You can listen on the same multiaddr as many times as you want
// (although only *one* listener will end up handling the inbound connection).
func (t *Transport) Listen(laddr ma.Multiaddr) (manet.Listener, error) {
	nw, naddr, err := manet.DialArgs(laddr)
	if err != nil {
		return nil, err
	}
	var n *network
	switch nw {
	case "tcp4":
		n = &t.v4
	case "tcp6":
		n = &t.v6
	default:
		return nil, ErrWrongListenProto
	}

	if !reuseport.Available() {
		return manet.Listen(laddr)
	}
	nl, err := reuseport.Listen(nw, naddr)
	if err != nil {
		return manet.Listen(laddr)
	}

	if _, ok := nl.Addr().(*net.TCPAddr); !ok {
		nl.Close()
		return nil, ErrWrongListenProto
	}

	malist, err := manet.WrapNetListener(nl)
	if err != nil {
		nl.Close()
		return nil, err
	}

	list := &listener{
		Listener: malist,
		network:  n,
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if n.tcpListeners == nil {
		n.tcpListeners = make(map[*listener]struct{})
	}
	n.tcpListeners[list] = struct{}{}
	n.tcpDialer = nil

	return list, nil
}

// ListenPacket is the UDP equivalent of `Listen`
func (t *Transport) ListenPacket(laddr ma.Multiaddr) (manet.PacketConn, error) {
	nw, naddr, err := manet.DialArgs(laddr)
	if err != nil {
		return nil, err
	}
	var n *network
	switch nw {
	case "udp4":
		n = &t.v4
	case "udp6":
		n = &t.v6
	default:
		return nil, ErrWrongListenPacketProto
	}

	if !reuseport.Available() {
		return manet.ListenPacket(laddr)
	}
	nl, err := reuseport.ListenPacket(nw, naddr)
	if err != nil {
		return manet.ListenPacket(laddr)
	}

	if _, ok := nl.LocalAddr().(*net.UDPAddr); !ok {
		nl.Close()
		return nil, ErrWrongListenPacketProto
	}

	malist, err := manet.WrapPacketConn(nl)
	if err != nil {
		nl.Close()
		return nil, err
	}

	list := &udpListener{
		PacketConn: malist,
		network:    n,
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if n.udpListeners == nil {
		n.udpListeners = make(map[*udpListener]struct{})
	}
	n.udpListeners[list] = struct{}{}
	n.udpDialer = nil

	return list, nil
}

package reusetransport

import (
	"errors"
	"sync"

	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("reuseport-transport")

// ErrWrongListenProto is returned when listen a protocol other than tcp.
var ErrWrongListenProto = errors.New("can only listen TCP over IPv4 or IPv6")

// ErrWrongListenPacketProto is returned when listen a protocol other than udp.
var ErrWrongListenPacketProto = errors.New("can only listen UDP packet over IPv4 or IPv6")

// ErrWrongDialProto is returned when dialing a protocol we cannot handle
var ErrWrongDialProto = errors.New("can only dial tcp4, tcp6, udp4, udp6")

// Transport is a reuse transport that reuses listener ports.
type Transport struct {
	v4 network
	v6 network
}

type network struct {
	mu           sync.RWMutex
	tcpListeners map[*listener]struct{}
	udpListeners map[*udpListener]struct{}
	tcpDialer    dialer
	udpDialer    dialer
}

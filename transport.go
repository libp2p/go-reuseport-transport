// Package tcpreuse provides a basic transport for automatically (and intelligently) reusing TCP ports.
//
// To use, construct a new Transport and configure listeners tr.Listen(...).
// When dialing (tr.Dial(...)), the transport will attempt to reuse the ports it's currently listening on,
// choosing the best one depending on the destination address.
//
// It is recommended to set set SO_LINGER to 0 for all connections, otherwise
// reusing the port may fail when re-dialing a recently closed connection.
// See https://hea-www.harvard.edu/~fine/Tech/addrinuse.html for details.
//
// Deprecated: This package has moved into go-libp2p as a sub-package: github.com/libp2p/go-libp2p/p2p/net/reuseport.
package tcpreuse

import (
	"github.com/libp2p/go-libp2p/p2p/net/reuseport"
)

// ErrWrongProto is returned when dialing a protocol other than tcp.
// Deprecated: use github.com/libp2p/go-libp2p/p2p/net/reuseport.ErrWrongProto instead.
var ErrWrongProto = reuseport.ErrWrongProto

// Transport is a TCP reuse transport that reuses listener ports.
// The zero value is safe to use.
// Deprecated: use github.com/libp2p/go-libp2p/p2p/net/reuseport.Transport instead.
type Transport = reuseport.Transport

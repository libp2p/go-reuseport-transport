package reusetransport

import (
	"errors"
	"net"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

var loopbackV4, _ = ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
var loopbackV6, _ = ma.NewMultiaddr("/ip6/::1/tcp/0")
var unspecV6, _ = ma.NewMultiaddr("/ip6/::/tcp/0")
var unspecV4, _ = ma.NewMultiaddr("/ip4/0.0.0.0/tcp/0")

var udpLoopbackV4, _ = ma.NewMultiaddr("/ip4/127.0.0.1/udp/0")
var udpLoopbackV6, _ = ma.NewMultiaddr("/ip6/::1/udp/0")
var udpUnspecV6, _ = ma.NewMultiaddr("/ip6/::/udp/0")
var udpUnspecV4, _ = ma.NewMultiaddr("/ip4/0.0.0.0/udp/0")

var udpMsg = []byte("udp-reuse-port-transport-test")
var udpMsgSize = len(udpMsg)
var errBadMsgSize = errors.New("bad message size")

var globalV4 ma.Multiaddr
var globalV6 ma.Multiaddr
var udpGlobalV4 ma.Multiaddr
var udpGlobalV6 ma.Multiaddr

func init() {
	addrs, err := manet.InterfaceMultiaddrs()
	if err != nil {
		return
	}
	for _, addr := range addrs {
		if !manet.IsIP6LinkLocal(addr) && !manet.IsIPLoopback(addr) {
			tcp, _ := ma.NewMultiaddr("/tcp/0")
			udp, _ := ma.NewMultiaddr("/udp/0")
			switch addr.Protocols()[0].Code {
			case ma.P_IP4:
				if globalV4 == nil {
					globalV4 = addr.Encapsulate(tcp)
				}
				if udpGlobalV4 == nil {
					udpGlobalV4 = addr.Encapsulate(udp)
				}
			case ma.P_IP6:
				if globalV6 == nil {
					globalV6 = addr.Encapsulate(tcp)
				}
				if udpGlobalV6 == nil {
					udpGlobalV6 = addr.Encapsulate(udp)
				}
			}
		}
	}
}

func acceptOne(t *testing.T, listener manet.Listener) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := listener.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		c.Close()
	}()
	return done
}

func udpAcceptOne(t *testing.T, listener manet.PacketConn) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buffer := make([]byte, udpMsgSize+1) // +1 in case we receive larger message than expected
		n, _, err := listener.ReadFrom(buffer)
		if err != nil {
			t.Error(err)
			return
		}
		if n != udpMsgSize {
			t.Error(errBadMsgSize)
			return
		}
	}()
	return done
}

func dialOne(t *testing.T, tr *Transport, listener manet.Listener, expected ...int) int {
	t.Helper()

	done := acceptOne(t, listener)
	c, err := tr.Dial(listener.Multiaddr())
	if err != nil {
		t.Fatal(err)
	}
	port := c.LocalAddr().(*net.TCPAddr).Port
	<-done
	c.Close()
	if len(expected) == 0 {
		return port
	}
	for _, p := range expected {
		if p == port {
			return port
		}
	}
	t.Errorf("dialed from %d, expected to dial from one of %v", port, expected)
	return 0
}

func udpDialOne(t *testing.T, tr *Transport, listener manet.PacketConn, expected ...int) int {
	t.Helper()

	done := udpAcceptOne(t, listener)
	c, err := tr.Dial(listener.Multiaddr())
	if err != nil {
		t.Fatal(err)
	}
	n, err := c.Write(udpMsg)
	if err != nil {
		t.Fatal(err)
	}
	if n != udpMsgSize {
		t.Fatal(errBadMsgSize)
	}
	port := c.LocalAddr().(*net.UDPAddr).Port
	<-done
	c.Close()
	if len(expected) == 0 {
		return port
	}
	for _, p := range expected {
		if p == port {
			return port
		}
	}
	t.Errorf("dialed from %d, expected to dial from one of %v", port, expected)
	return 0
}

func TestNoneAndSingle(t *testing.T) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(loopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	dialOne(t, &trB, listenerA)

	listenerB, err := trB.Listen(loopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB.Close()

	dialOne(t, &trB, listenerA, listenerB.Addr().(*net.TCPAddr).Port)
}

func TestUdpNoneAndSingle(t *testing.T) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.ListenPacket(udpLoopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	udpDialOne(t, &trB, listenerA)

	listenerB, err := trB.ListenPacket(udpLoopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB.Close()

	udpDialOne(t, &trB, listenerA, listenerB.Connection().LocalAddr().(*net.UDPAddr).Port)
}

func TestTwoLocal(t *testing.T) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(loopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.Listen(loopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	listenerB2, err := trB.Listen(loopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	dialOne(t, &trB, listenerA,
		listenerB1.Addr().(*net.TCPAddr).Port,
		listenerB2.Addr().(*net.TCPAddr).Port)
}

func TestUdpTwoLocal(t *testing.T) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.ListenPacket(udpLoopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.ListenPacket(udpLoopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	listenerB2, err := trB.ListenPacket(udpLoopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	udpDialOne(t, &trB, listenerA,
		listenerB1.Connection().LocalAddr().(*net.UDPAddr).Port,
		listenerB2.Connection().LocalAddr().(*net.UDPAddr).Port)
}

func TestGlobalPreferenceV4(t *testing.T) {
	if globalV4 == nil {
		t.Skip("no global IPv4 addresses configured")
		return
	}
	testPrefer(t, loopbackV4, loopbackV4, globalV4)
	testPrefer(t, loopbackV4, unspecV4, globalV4)

	testPrefer(t, globalV4, unspecV4, globalV4)
	testPrefer(t, globalV4, unspecV4, loopbackV4)

	if udpGlobalV4 == nil {
		t.Skip("no global IPv4 addresses configured")
		return
	}
	testUdpPrefer(t, udpLoopbackV4, udpLoopbackV4, udpGlobalV4)
	testUdpPrefer(t, udpLoopbackV4, udpUnspecV4, udpGlobalV4)

	testUdpPrefer(t, udpGlobalV4, udpUnspecV4, udpGlobalV4)
	testUdpPrefer(t, udpGlobalV4, udpUnspecV4, udpLoopbackV4)
}

func TestGlobalPreferenceV6(t *testing.T) {
	if globalV6 == nil {
		t.Skip("no global IPv6 addresses configured")
		return
	}
	testPrefer(t, loopbackV6, loopbackV6, globalV6)
	testPrefer(t, loopbackV6, unspecV6, globalV6)

	testPrefer(t, globalV6, unspecV6, globalV6)
	testPrefer(t, globalV6, unspecV6, loopbackV6)

	if udpGlobalV6 == nil {
		t.Skip("no global IPv6 addresses configured")
		return
	}
	testUdpPrefer(t, udpLoopbackV6, udpLoopbackV6, udpGlobalV6)
	testUdpPrefer(t, udpLoopbackV6, udpUnspecV6, udpGlobalV6)

	testUdpPrefer(t, udpGlobalV6, udpUnspecV6, udpGlobalV6)
	testUdpPrefer(t, udpGlobalV6, udpUnspecV6, udpLoopbackV6)
}

func TestLoopbackPreference(t *testing.T) {
	testPrefer(t, loopbackV4, loopbackV4, unspecV4)
	testPrefer(t, loopbackV6, loopbackV6, unspecV6)

	testUdpPrefer(t, udpLoopbackV4, udpLoopbackV4, udpUnspecV4)
	testUdpPrefer(t, udpLoopbackV6, udpLoopbackV6, udpUnspecV6)
}

func testPrefer(t *testing.T, listen, prefer, avoid ma.Multiaddr) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(listen)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.Listen(avoid)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	dialOne(t, &trB, listenerA, listenerB1.Addr().(*net.TCPAddr).Port)

	listenerB2, err := trB.Listen(prefer)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	dialOne(t, &trB, listenerA, listenerB2.Addr().(*net.TCPAddr).Port)

	// Closing the listener should reset the dialer.
	listenerB2.Close()

	dialOne(t, &trB, listenerA, listenerB1.Addr().(*net.TCPAddr).Port)
}

func testUdpPrefer(t *testing.T, listen, prefer, avoid ma.Multiaddr) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.ListenPacket(listen)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.ListenPacket(avoid)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	udpDialOne(t, &trB, listenerA, listenerB1.Connection().LocalAddr().(*net.UDPAddr).Port)

	listenerB2, err := trB.ListenPacket(prefer)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	udpDialOne(t, &trB, listenerA, listenerB2.Connection().LocalAddr().(*net.UDPAddr).Port)

	// Closing the listener should reset the dialer.
	listenerB2.Close()

	udpDialOne(t, &trB, listenerA, listenerB1.Connection().LocalAddr().(*net.UDPAddr).Port)
}

func TestV6V4(t *testing.T) {
	testUseFirst(t, loopbackV4, loopbackV4, loopbackV6)
	testUseFirst(t, loopbackV6, loopbackV6, loopbackV4)

	testUdpUseFirst(t, udpLoopbackV4, udpLoopbackV4, udpLoopbackV6)
	testUdpUseFirst(t, udpLoopbackV6, udpLoopbackV6, udpLoopbackV4)
}

func TestGlobalToGlobal(t *testing.T) {
	if globalV4 == nil {
		t.Skip("no globalV4 addresses configured")
		return
	}
	testUseFirst(t, globalV4, globalV4, loopbackV4)
	testUseFirst(t, globalV6, globalV6, loopbackV6)

	if udpGlobalV4 == nil {
		t.Skip("no globalV4 addresses configured")
		return
	}
	testUdpUseFirst(t, udpGlobalV4, udpGlobalV4, udpLoopbackV4)
	testUdpUseFirst(t, udpGlobalV6, udpGlobalV6, udpLoopbackV6)
}

func testUseFirst(t *testing.T, listen, use, never ma.Multiaddr) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(globalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.Listen(loopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	// It works (random port)
	dialOne(t, &trB, listenerA)

	listenerB2, err := trB.Listen(globalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	// Uses globalV4 port.
	dialOne(t, &trB, listenerA, listenerB2.Addr().(*net.TCPAddr).Port)

	// Closing the listener should reset the dialer.
	listenerB2.Close()

	// It still works.
	dialOne(t, &trB, listenerA)
}

func testUdpUseFirst(t *testing.T, listen, use, never ma.Multiaddr) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.ListenPacket(udpGlobalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.ListenPacket(udpLoopbackV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	// It works (random port)
	udpDialOne(t, &trB, listenerA)

	listenerB2, err := trB.ListenPacket(udpGlobalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	// Uses globalV4 port.
	udpDialOne(t, &trB, listenerA, listenerB2.Connection().LocalAddr().(*net.UDPAddr).Port)

	// Closing the listener should reset the dialer.
	listenerB2.Close()

	// It still works.
	udpDialOne(t, &trB, listenerA)
}

func TestDuplicateGlobal(t *testing.T) {
	if globalV4 == nil {
		t.Skip("no globalV4 addresses configured")
		return
	}

	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(globalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.Listen(globalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	listenerB2, err := trB.Listen(globalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	// Check which port we're using
	port := dialOne(t, &trB, listenerA)

	// Check consistency
	for i := 0; i < 10; i++ {
		dialOne(t, &trB, listenerA, port)
	}
}

func TestUdpDuplicateGlobal(t *testing.T) {
	if udpGlobalV4 == nil {
		t.Skip("no globalV4 addresses configured")
		return
	}

	var trA Transport
	var trB Transport
	listenerA, err := trA.ListenPacket(udpGlobalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.ListenPacket(udpGlobalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	listenerB2, err := trB.ListenPacket(udpGlobalV4)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	// Check which port we're using
	port := udpDialOne(t, &trB, listenerA)

	// Check consistency
	for i := 0; i < 10; i++ {
		udpDialOne(t, &trB, listenerA, port)
	}
}

package tcpreuse

import (
	"net"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

var loopback, _ = ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
var unspec, _ = ma.NewMultiaddr("/ip4/0.0.0.0/tcp/0")

var global ma.Multiaddr

func init() {
	addrs, err := manet.InterfaceMultiaddrs()
	if err != nil {
		return
	}
	for _, addr := range addrs {
		if !manet.IsIP6LinkLocal(addr) && !manet.IsIPLoopback(addr) {
			tcp, _ := ma.NewMultiaddr("/tcp/0")
			global = addr.Encapsulate(tcp)
			return
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

func TestNoneAndSingle(t *testing.T) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(loopback)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	dialOne(t, &trB, listenerA)

	listenerB, err := trB.Listen(loopback)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB.Close()

	dialOne(t, &trB, listenerA, listenerB.Addr().(*net.TCPAddr).Port)
}

func TestTwoLocal(t *testing.T) {
	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(loopback)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.Listen(loopback)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	listenerB2, err := trB.Listen(loopback)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	dialOne(t, &trB, listenerA,
		listenerB1.Addr().(*net.TCPAddr).Port,
		listenerB2.Addr().(*net.TCPAddr).Port)
}

func TestGlobalPreference(t *testing.T) {
	if global == nil {
		t.Skip("no global addresses configured")
		return
	}
	testPrefer(t, loopback, loopback, global)
	testPrefer(t, loopback, unspec, global)

	testPrefer(t, global, unspec, global)
	testPrefer(t, global, unspec, loopback)
}

func TestLoopbackPreference(t *testing.T) {
	testPrefer(t, loopback, loopback, unspec)
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

func TestGlobalToGlobal(t *testing.T) {
	if global == nil {
		t.Skip("no global addresses configured")
		return
	}

	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(global)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.Listen(loopback)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	// It works (random port)
	dialOne(t, &trB, listenerA)

	listenerB2, err := trB.Listen(global)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB2.Close()

	// Uses global port.
	dialOne(t, &trB, listenerA, listenerB2.Addr().(*net.TCPAddr).Port)

	// Closing the listener should reset the dialer.
	listenerB2.Close()

	// It still works.
	dialOne(t, &trB, listenerA)
}

func TestDuplicateGlobal(t *testing.T) {
	if global == nil {
		t.Skip("no global addresses configured")
		return
	}

	var trA Transport
	var trB Transport
	listenerA, err := trA.Listen(global)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerA.Close()

	listenerB1, err := trB.Listen(global)
	if err != nil {
		t.Fatal(err)
	}
	defer listenerB1.Close()

	listenerB2, err := trB.Listen(global)
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

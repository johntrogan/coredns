package file

import (
	"fmt"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestLess(t *testing.T) {
	const (
		min  = 0
		max  = 4294967295
		low  = 12345
		high = 4000000000
	)

	if less(min, max) {
		t.Fatalf("Less: should be false")
	}
	if !less(max, min) {
		t.Fatalf("Less: should be true")
	}
	if !less(high, low) {
		t.Fatalf("Less: should be true")
	}
	if !less(7, 9) {
		t.Fatalf("Less; should be true")
	}
}

type soa struct {
	serial uint32
}

func (s *soa) Handler(w dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)
	switch req.Question[0].Qtype {
	case dns.TypeSOA:
		m.Answer = make([]dns.RR, 1)
		m.Answer[0] = test.SOA(fmt.Sprintf("%s IN SOA bla. bla. %d 0 0 0 0 ", testZone, s.serial))
		w.WriteMsg(m)
	case dns.TypeAXFR:
		m.Answer = make([]dns.RR, 4)
		m.Answer[0] = test.SOA(fmt.Sprintf("%s IN SOA bla. bla. %d 0 0 0 0 ", testZone, s.serial))
		m.Answer[1] = test.A(fmt.Sprintf("%s IN A 127.0.0.1", testZone))
		m.Answer[2] = test.A(fmt.Sprintf("%s IN A 127.0.0.1", testZone))
		m.Answer[3] = test.SOA(fmt.Sprintf("%s IN SOA bla. bla. %d 0 0 0 0 ", testZone, s.serial))
		w.WriteMsg(m)
	}
}

func (s *soa) TransferHandler(w dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)
	m.Answer = make([]dns.RR, 1)
	m.Answer[0] = test.SOA(fmt.Sprintf("%s IN SOA bla. bla. %d 0 0 0 0 ", testZone, s.serial))
	w.WriteMsg(m)
}

const testZone = "secondary.miek.nl."

func TestShouldTransfer(t *testing.T) {
	soa := soa{250}

	s := dnstest.NewServer(soa.Handler)
	defer s.Close()

	z := NewZone("testzone", "test")
	z.origin = testZone
	z.TransferFrom = []string{s.Addr}

	// when we have a nil SOA (initial state)
	should, err := z.shouldTransfer()
	if err != nil {
		t.Fatalf("Unable to run shouldTransfer: %v", err)
	}
	if !should {
		t.Fatalf("ShouldTransfer should return true for serial: %d", soa.serial)
	}
	// Serial smaller
	z.SOA = test.SOA(fmt.Sprintf("%s IN SOA bla. bla. %d 0 0 0 0 ", testZone, soa.serial-1))
	should, err = z.shouldTransfer()
	if err != nil {
		t.Fatalf("Unable to run shouldTransfer: %v", err)
	}
	if !should {
		t.Fatalf("ShouldTransfer should return true for serial: %q", soa.serial-1)
	}
	// Serial equal
	z.SOA = test.SOA(fmt.Sprintf("%s IN SOA bla. bla. %d 0 0 0 0 ", testZone, soa.serial))
	should, err = z.shouldTransfer()
	if err != nil {
		t.Fatalf("Unable to run shouldTransfer: %v", err)
	}
	if should {
		t.Fatalf("ShouldTransfer should return false for serial: %d", soa.serial)
	}
}

func TestTransferIn(t *testing.T) {
	soa := soa{250}

	s := dnstest.NewServer(soa.Handler)
	defer s.Close()

	z := new(Zone)
	z.origin = testZone
	z.TransferFrom = []string{s.Addr}

	if err := z.TransferIn(); err != nil {
		t.Fatalf("Unable to run TransferIn: %v", err)
	}
	if z.SOA.String() != fmt.Sprintf("%s	3600	IN	SOA	bla. bla. 250 0 0 0 0", testZone) {
		t.Fatalf("Unknown SOA transferred")
	}
}

func TestIsNotify(t *testing.T) {
	z := new(Zone)
	z.origin = testZone
	state := newRequest(testZone, dns.TypeSOA)
	// need to set opcode
	state.Req.Opcode = dns.OpcodeNotify

	z.TransferFrom = []string{"10.240.0.1:53"} // IP from testing/responseWriter
	if !z.isNotify(state) {
		t.Fatal("Should have been valid notify")
	}
	z.TransferFrom = []string{"10.240.0.2:53"}
	if z.isNotify(state) {
		t.Fatal("Should have been invalid notify")
	}
}

func newRequest(zone string, qtype uint16) request.Request {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	m.SetEdns0(4097, true)
	return request.Request{W: &test.ResponseWriter{}, Req: m}
}

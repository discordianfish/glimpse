package main

import (
	"net"
	"reflect"
	"testing"

	"github.com/miekg/dns"
)

func TestDNSHandler(t *testing.T) {
	var (
		s = &testStore{}
		h = dnsHandler(s, "tt", "srv.glimpse.io")
		w = &testWriter{}
	)

	s.instances = []*instance{
		{
			info: info{
				service: "http",
				job:     "api",
				env:     "prod",
				product: "harpoon",
				zone:    "tt",
			},
			host: "localhost",
			ip:   net.ParseIP("127.0.0.1"),
			port: uint16(12345),
		},
	}

	m := &dns.Msg{}
	m.SetQuestion("http.api.prod.harpoon.", dns.TypeSRV)
	h(w, m)
	r := w.msg

	if want, got := dns.RcodeSuccess, m.Rcode; want != got {
		t.Errorf("want %d rcode, got %d\n", want, got)
	}

	if want, got := 1, len(r.Answer); want != got {
		t.Errorf("want %d answers, got %d\n", want, got)
	}
}

// testStore implements the glimpse.store interface.
type testStore struct {
	instances []*instance
}

func (s *testStore) getInstances(srv info) (instances, error) {
	var r instances

	for _, i := range s.instances {
		if reflect.DeepEqual(srv, i.info) {
			r = append(r, i)
		}
	}

	return r, nil
}

// testWriter implements the dns.ResponseWriter interface.
type testWriter struct {
	msg *dns.Msg
}

func (w *testWriter) WriteMsg(m *dns.Msg) error {
	w.msg = m
	return nil
}

func (w *testWriter) LocalAddr() net.Addr         { return nil }
func (w *testWriter) RemoteAddr() net.Addr        { return nil }
func (w *testWriter) Write(s []byte) (int, error) { return 0, nil }
func (w *testWriter) Close() error                { return nil }
func (w *testWriter) TsigStatus() error           { return nil }
func (w *testWriter) TsigTimersOnly(b bool)       {}
func (w *testWriter) Hijack()                     {}

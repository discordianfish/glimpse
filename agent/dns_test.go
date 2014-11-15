package main

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/miekg/dns"
)

func TestDNSHandler(t *testing.T) {
	var (
		domain = "srv.glimpse.io"
		zone   = "tt"

		api = info{
			service: "http",
			job:     "api",
			env:     "prod",
			product: "harpoon",
			zone:    zone,
		}
		web = info{
			service: "http",
			job:     "web",
			env:     "prod",
			product: "harpoon",
			zone:    zone,
		}

		store = &testStore{
			instances: instances{
				{
					info: api,
					host: "host1",
					ip:   net.ParseIP("127.0.0.1"),
					port: uint16(20000),
				},
				{
					info: api,
					host: "host1",
					ip:   net.ParseIP("127.0.0.1"),
					port: uint16(20001),
				},
				{
					info: api,
					host: "host2",
					ip:   net.ParseIP("127.0.0.2"),
					port: uint16(20000),
				},
				{
					info: api,
					host: "host2",
					ip:   net.ParseIP("127.0.0.2"),
					port: uint16(20003),
				},
				{
					info: web,
					host: "host3",
					ip:   net.ParseIP("127.0.0.3"),
					port: uint16(21000),
				},
				{
					info: web,
					host: "host4",
					ip:   net.ParseIP("127.0.0.4"),
					port: uint16(21003),
				},
			},
		}

		h = dnsHandler(store, zone, domain)
		w = &testWriter{}
	)

	for _, test := range []struct {
		question string
		answers  int
		rcode    int
	}{
		{
			question: fmt.Sprintf("http.api.prod.harpoon.%s.%s.", zone, domain),
			answers:  4,
		},
		{
			question: fmt.Sprintf("http.web.prod.harpoon.%s.%s.", zone, domain),
			answers:  2,
		},
	} {
		m := &dns.Msg{}
		m.SetQuestion(test.question, dns.TypeSRV)

		h(w, m)
		r := w.msg

		if want, got := test.rcode, r.Rcode; want != got {
			t.Errorf("want rcode %s, got %s", dns.RcodeToString[want], dns.RcodeToString[got])
		}

		if want, got := false, r.RecursionAvailable; want != got {
			t.Errorf("want available recursion %t, got %t", want, got)
		}

		if want, got := test.answers, len(r.Answer); want != got {
			t.Errorf("want %d answers, got %d\n", want, got)
		}
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

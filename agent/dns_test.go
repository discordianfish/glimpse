package main

import (
	"fmt"
	"math/rand"
	"net"
	"testing"

	"github.com/miekg/dns"
)

func TestDNSHandler(t *testing.T) {
	var (
		domain = dns.Fqdn("srv.glimpse.io")
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
			instances: map[info]instances{
				api: instances{
					{
						host: "host1",
						ip:   net.ParseIP("127.0.0.1"),
						port: uint16(20000),
					},
					{
						host: "host1",
						ip:   net.ParseIP("127.0.0.1"),
						port: uint16(20001),
					},
					{
						host: "host2",
						ip:   net.ParseIP("127.0.0.2"),
						port: uint16(20000),
					},
					{
						host: "host2",
						ip:   net.ParseIP("127.0.0.2"),
						port: uint16(20003),
					},
				},
				web: instances{
					{
						host: "host3",
						ip:   net.ParseIP("127.0.0.3"),
						port: uint16(21000),
					},
					{
						host: "host4",
						ip:   net.ParseIP("127.0.0.4"),
						port: uint16(21003),
					},
				},
			},
			servers: map[string]instances{
				zone: instances{{host: "foo"}},
			},
		}

		h = dnsHandler(store, zone, domain)
		w = &testWriter{}
	)

	for _, test := range []struct {
		question string
		qtype    uint16
		answers  int
		rcode    int
	}{
		{
			question: fmt.Sprintf("foo.bar.baz.qux.%s.%s", zone, domain),
			qtype:    dns.TypeSRV,
			rcode:    dns.RcodeNameError,
		},
		{
			question: fmt.Sprintf("foo.bar.baz.qux.%s.%s", "invalid", domain),
			qtype:    dns.TypeSRV,
			rcode:    dns.RcodeNameError,
		},
		{
			question: "http.api.prod.harpoon.",
			qtype:    dns.TypeSRV,
			rcode:    dns.RcodeNameError,
		},
		{
			question: fmt.Sprintf("http.api.prod.harpoon.%s", zone),
			qtype:    dns.TypeSRV,
			rcode:    dns.RcodeNameError,
		},
		{
			question: fmt.Sprintf("http.api.prod.harpoon.%s.", zone),
			qtype:    dns.TypeSRV,
			rcode:    dns.RcodeNameError,
		},
		{
			question: fmt.Sprintf("http.api.prod.harpoon.%s.%s", zone, domain),
			qtype:    dns.TypeSRV,
			answers:  4,
		},
		{
			question: fmt.Sprintf("http.web.prod.harpoon.%s.%s", zone, domain),
			qtype:    dns.TypeSRV,
			answers:  2,
		},
		{
			question: fmt.Sprintf("foo.bar.baz.qux.%s.%s", zone, domain),
			qtype:    dns.TypeA,
			rcode:    dns.RcodeNameError,
		},
		{
			question: fmt.Sprintf("http.api.prod.harpoon.%s.%s", zone, domain),
			qtype:    dns.TypeA,
			answers:  4,
		},
		{
			question: fmt.Sprintf("http.web.prod.harpoon.%s.%s", zone, domain),
			qtype:    dns.TypeA,
			answers:  2,
		},
		{
			question: fmt.Sprintf("%s.%s", zone, domain),
			qtype:    dns.TypeNS,
			answers:  1,
		},
		{
			question: fmt.Sprintf("xx.%s", domain),
			qtype:    dns.TypeNS,
		},
		{
			question: fmt.Sprintf("foo.%s.%s", zone, domain),
			qtype:    dns.TypeNS,
			rcode:    dns.RcodeNameError,
		},
		{
			question: fmt.Sprintf("http.web.prod.harpoon.%s.%s", zone, domain),
			qtype:    dns.TypeAAAA,
			rcode:    dns.RcodeNotImplemented,
		},
		{
			question: fmt.Sprintf("http.web.prod.harpoon.%s.%s", zone, domain),
			qtype:    dns.TypeMX,
			rcode:    dns.RcodeNotImplemented,
		},
		{
			question: fmt.Sprintf("http.web.prod.harpoon.%s.%s", zone, domain),
			qtype:    dns.TypeTXT,
			rcode:    dns.RcodeNotImplemented,
		},
	} {
		m := &dns.Msg{}
		m.SetQuestion(test.question, test.qtype)

		h(w, m)
		r := w.msg

		if want, got := test.rcode, r.Rcode; want != got {
			t.Errorf("want rcode %s, got %s", dns.RcodeToString[want], dns.RcodeToString[got])
		}

		if want, got := false, r.RecursionAvailable; want != got {
			t.Errorf("want available recursion %t, got %t", want, got)
		}

		if want, got := r.Rcode == dns.RcodeSuccess, r.Authoritative; want != got {
			t.Errorf("want authoritative %t, got %t", want, got)
		}

		if want, got := test.answers, len(r.Answer); want != got {
			t.Errorf("want %d answers, got %d\n", want, got)
		}

		for _, answer := range r.Answer {
			switch test.qtype {
			case dns.TypeA:
				_, ok := answer.(*dns.A)
				if !ok {
					t.Error("want A resource record, got something else")
				}
			case dns.TypeSRV:
				_, ok := answer.(*dns.SRV)
				if !ok {
					t.Error("want SRV resource record, got something else")
				}
			}
		}
	}
}

func TestDNSHandlerZeroQuestions(t *testing.T) {
	var (
		h = dnsHandler(&testStore{}, "tt", dns.Fqdn("test.glimpse.io"))
		m = &dns.Msg{}
		w = &testWriter{}
	)

	h(w, m)
	r := w.msg

	if want, got := dns.RcodeFormatError, r.Rcode; want != got {
		t.Errorf("want rcode %s, got %s", dns.RcodeToString[want], dns.RcodeToString[got])
	}
}

func TestDNSHandlerMultiQuestions(t *testing.T) {
	var (
		h = dnsHandler(&testStore{}, "tt", dns.Fqdn("test.glimpse.io"))
		m = &dns.Msg{}
		w = &testWriter{}
	)

	m.Id = dns.Id()
	m.RecursionDesired = true
	m.Question = make([]dns.Question, 3)
	for i := range m.Question {
		m.Question[i] = dns.Question{
			Name:   "foo.bar.baz.",
			Qtype:  dns.TypeA,
			Qclass: dns.ClassINET,
		}
	}

	h(w, m)
	r := w.msg

	if want, got := dns.RcodeNotImplemented, r.Rcode; want != got {
		t.Errorf("want rcode %s, got %s", dns.RcodeToString[want], dns.RcodeToString[got])
	}
}

func TestDNSHandlerBrokenStore(t *testing.T) {
	var (
		h = dnsHandler(&brokenStore{}, "tt", dns.Fqdn("test.glimpse.io"))
		m = &dns.Msg{}
		w = &testWriter{}
	)

	m.SetQuestion("http.api.prod.harpoon.tt.test.glimpse.io.", dns.TypeSRV)
	h(w, m)
	r := w.msg

	if want, got := dns.RcodeServerFailure, r.Rcode; want != got {
		t.Errorf("want rcode %s, got %s", dns.RcodeToString[want], dns.RcodeToString[got])
	}
}

func TestProtocolHandler(t *testing.T) {
	var (
		answers     = rand.Intn(12) + 3
		want        = answers / 2
		testHandler = dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			res := &dns.Msg{}

			for i := 0; i < answers; i++ {
				rr := &dns.A{
					Hdr: dns.RR_Header{
						Name:   r.Question[0].Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    5,
					},
					A: net.ParseIP(fmt.Sprintf("1.2.3.%d", i)),
				}
				res.Answer = append(res.Answer, rr)
			}

			err := w.WriteMsg(res)
			if err != nil {
				t.Fatalf("write response failed: %s", err)
			}
		})
	)

	w := &testWriter{
		remoteAddr: &net.UDPAddr{},
	}
	m := &dns.Msg{}
	m.SetQuestion(dns.Fqdn("app.glimpse.io"), dns.TypeA)

	protocolHandler(want, testHandler).ServeDNS(w, m)

	if got := len(w.msg.Answer); want != got {
		t.Errorf("want %d answers, got %d", want, got)
	}
}

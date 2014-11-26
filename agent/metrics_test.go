package main

import (
	"net"
	"reflect"
	"testing"

	"github.com/miekg/dns"
)

func TestDnsMetricsHandler(t *testing.T) {
	testHandler := dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		res := &dns.Msg{}
		res.SetReply(req)
		res.SetRcode(req, dns.RcodeNotImplemented)

		err := w.WriteMsg(res)
		if err != nil {
			t.Fatalf("write response failed: %s", err)
		}
	})

	w := &testWriter{
		remoteAddr: &net.UDPAddr{},
	}

	m := &dns.Msg{}
	m.SetQuestion(dns.Fqdn("app.glimpse.io"), dns.TypeA)

	dnsMetricsHandler(testHandler).ServeDNS(w, m)
	r := w.msg

	if want, got := dns.RcodeNotImplemented, r.Rcode; want != got {
		t.Errorf("want rcode %s, got %s", dns.RcodeToString[want], dns.RcodeToString[got])
	}
}

func TestMetricsStore(t *testing.T) {
	var (
		i = info{
			service: "http",
			job:     "walker",
			env:     "prod",
			product: "harpoon",
			zone:    "tt",
		}
		ins = generateInstancesFromInfo(i)
		s   = newMetricsStore(&testStore{instances: ins})
	)

	sins, err := s.getInstances(i)
	if err != nil {
		t.Fatalf("want store to not return an error, got %s", err)
	}

	if want, got := ins, sins; !reflect.DeepEqual(want, got) {
		t.Errorf("want %d instances, got %d", len(want), len(got))
	}
}

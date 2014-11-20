package main

import (
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

	w := &testWriter{}

	m := &dns.Msg{}
	m.SetQuestion(dns.Fqdn("app.glimpse.io"), dns.TypeA)

	dnsMetricsHandler(testHandler).ServeDNS(w, m)
	r := w.msg

	if want, got := dns.RcodeNotImplemented, r.Rcode; want != got {
		t.Errorf("want rcode %s, got %s", dns.RcodeToString[want], dns.RcodeToString[got])
	}
}

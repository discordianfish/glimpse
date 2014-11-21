package main

import (
	"math/rand"
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

	w := &testWriter{}

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
	)

	sins, err := newMetricsStore(&testStore{instances: ins}).getInstances(i)
	if err != nil {
		t.Fatalf("want store to not return an error, got %s", err)
	}

	if want, got := ins, sins; !reflect.DeepEqual(want, got) {
		t.Errorf("want %d instances, got %d", len(want), len(got))
	}
}

func generateInstances(addrs ...string) (instances, error) {
	ins := instances{}

	for _, addr := range addrs {
		i, err := infoFromAddr(addr)
		if err != nil {
			return nil, err
		}

		ins = append(ins, generateInstancesFromInfo(i)...)
	}

	return ins, nil
}

func generateInstancesFromInfo(i info) instances {
	var (
		n   = rand.Intn(10) + 1
		ins = make(instances, n)
	)

	for j := 0; j < n; j++ {
		ins[j] = &instance{
			info: i,
			host: "suppenkasper",
			ip:   net.ParseIP("1.2.3.4"),
			port: uint16(20000 + j),
		}
	}

	return ins
}

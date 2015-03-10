package main

import (
	"bytes"
	"log"
	"net"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

func TestDNSLoggingHandler(t *testing.T) {
	var (
		b    = &bytes.Buffer{}
		l    = log.New(b, "glimpse-agent ", log.Lmicroseconds)
		fqdn = dns.Fqdn("db.glimpse.io")
		w    = &testWriter{
			remoteAddr: &net.UDPAddr{
				IP:   net.ParseIP("8.7.6.5"),
				Port: 4321,
			},
		}
		testHandler = dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
			res := &dns.Msg{}
			res.SetReply(req)
			res.SetRcode(req, dns.RcodeNotImplemented)

			err := w.WriteMsg(res)
			if err != nil {
				t.Fatalf("write response failed: %s", err)
			}
		})
	)

	m := &dns.Msg{}
	m.SetQuestion(fqdn, dns.TypeA)

	dnsLoggingHandler(l, testHandler).ServeDNS(w, m)

	sp := strings.SplitN(strings.Trim(b.String(), "\n"), " ", 10)

	if want, have := 9, len(sp); want != have {
		t.Fatalf("want %d fields, got %d fields", want, have)
	}

	if want, have := "DNS", sp[2]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := "8.7.6.5:4321", sp[4]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := "A", sp[5]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := fqdn, sp[6]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := dns.RcodeToString[dns.RcodeNotImplemented], sp[7]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := "0", sp[8]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}
}

func TestDNSLoggingHandlerError(t *testing.T) {
	var (
		b = &bytes.Buffer{}
		l = log.New(b, "glimpse-agent ", log.Lmicroseconds)
		m = &dns.Msg{}
		e = &errorWriter{
			&testWriter{
				remoteAddr: &net.UDPAddr{},
			},
		}
		errorHandler = dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
			err := w.WriteMsg(&dns.Msg{})
			if err == nil {
				t.Fatalf("want WriteMsg() to fail with errorWriter")
			}
		})
	)

	m.SetQuestion(dns.Fqdn("app.glimpse.io"), dns.TypeA)
	dnsLoggingHandler(l, errorHandler).ServeDNS(e, m)

	sp := strings.SplitN(strings.Trim(b.String(), "\n"), " ", 10)

	if want, have := "error: failed write", sp[9]; want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}
}

func TestLoggingStoreError(t *testing.T) {
	var (
		i = info{
			service: "xmpp",
			job:     "agent",
			env:     "qa",
			product: "mack",
			zone:    "de",
		}
		is = generateInstancesFromInfo(i)
		b  = &bytes.Buffer{}
		l  = log.New(b, "glimpse-agent ", log.Lmicroseconds)
		s  = newLoggingStore(l, &testStore{
			instances: map[info]instances{i: is},
			servers: map[string]instances{
				i.zone: is,
			},
		})
	)

	_, err := s.getInstances(i)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.getServers(i.zone)
	if err != nil {
		t.Fatal(err)
	}

	// Successfull transactions should not result in a log line.
	if want, have := 0, len(b.String()); want != have {
		t.Errorf("want %d, have %d", want, have)
	}

	_, err = s.getInstances(info{product: "nonsense"})
	if err == nil {
		t.Error("want loggingStore to pass errors")
	}

	sp := strings.Split(strings.Trim(b.String(), "\n"), " ")

	if want, have := "STORE", sp[2]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := "getInstances", sp[4]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := "...nonsense", sp[5]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := "error:", sp[6]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	b.Reset()

	_, err = s.getServers("zz")
	if err == nil {
		t.Errorf("want loggingStore to pass errors")
	}

	sp = strings.Split(strings.Trim(b.String(), "\n"), " ")

	if want, have := "STORE", sp[2]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := "getServers", sp[4]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := "zz", sp[5]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := errNoInstances.Error(), strings.Join(sp[7:], " "); strings.HasPrefix(want, have) {
		t.Errorf("want %s, have %s", want, have)
	}
}

package main

import (
	"log"
	"strings"

	"github.com/miekg/dns"
)

func nonExistentHandler() dns.HandlerFunc {
	return func(w dns.ResponseWriter, req *dns.Msg) {
		res := &dns.Msg{}
		res.SetReply(req)
		res.SetRcode(req, dns.RcodeNameError)

		err := w.WriteMsg(res)
		if err != nil {
			log.Printf("[warning] write msg failed: %s", err)
		}
	}
}

func dnsHandler(store store, zone, domain string) dns.HandlerFunc {
	return func(w dns.ResponseWriter, req *dns.Msg) {
		var (
			q   = req.Question[0]
			res = &dns.Msg{}
		)

		if len(req.Question) > 1 {
			log.Printf("warn: question > 1: %+v\n", req.Question)

			for _, q := range req.Question {
				log.Printf(
					"warn: %s %s %s\n",
					dns.TypeToString[q.Qtype],
					dns.ClassToString[q.Qclass],
					q.Name,
				)
			}
		}

		res.SetReply(req)
		res.Authoritative = true
		res.RecursionAvailable = false

		switch q.Qtype {
		case dns.TypeSRV:
			addr := q.Name

			// Trim domain as it is not relevant for the extraction from the
			// service address.
			addr = strings.TrimSuffix(addr, "."+domain+".")

			srv, err := infoFromAddr(addr)
			if err != nil {
				log.Printf("err: extract lookup '%s': %s", q.Name, err)
				res.SetRcode(req, dns.RcodeNameError)
				break
			}

			nodes, err := store.getInstances(srv)
			if err != nil {
				log.Fatalf("consul lookup failed: %s", err)
			}

			// TODO(ts): handle registered service without instances
			if len(nodes) == 0 {
				res.SetRcode(req, dns.RcodeNameError)
				break
			}

			for _, n := range nodes {
				rec := &dns.SRV{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeSRV,
						Class:  dns.ClassINET,
						Ttl:    5,
					},
					Priority: 0,
					Weight:   0,
					Port:     n.port,
					Target:   n.host + ".",
				}
				res.Answer = append(res.Answer, rec)
			}
		default:
			res.SetRcode(req, dns.RcodeNameError)
		}

		err := w.WriteMsg(res)
		if err != nil {
			log.Printf("[warning] write msg failed: %s", err)
		}

		// TODO(alx): Put logging in central place for control in different
		//						environemnts.
		log.Printf("query: %s %s -> %d\n", dns.TypeToString[q.Qtype], q.Name, len(res.Answer))
	}
}

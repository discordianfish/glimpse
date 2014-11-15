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
			instances instances

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

		// Trim domain as it is not relevant for the extraction from the
		// service address.
		addr := strings.TrimSuffix(q.Name, "."+domain+".")

		srv, err := infoFromAddr(addr)
		if err != nil {
			log.Printf("[warning] extract lookup '%s': %s", q.Name, err)
			res.SetRcode(req, dns.RcodeNameError)
			goto respond
		}

		instances, err = store.getInstances(srv)
		if err != nil {
			log.Fatalf("consul lookup failed: %s", err)
		}

		// TODO(ts): handle registered service without instances
		if len(instances) == 0 {
			res.SetRcode(req, dns.RcodeNameError)
			goto respond
		}

		switch q.Qtype {
		case dns.TypeA:
			for _, ins := range instances {
				rr := &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    5,
					},
					A: ins.ip,
				}
				res.Answer = append(res.Answer, rr)
			}
		case dns.TypeSRV:
			for _, ins := range instances {
				rr := &dns.SRV{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeSRV,
						Class:  dns.ClassINET,
						Ttl:    5,
					},
					Priority: 0,
					Weight:   0,
					Port:     ins.port,
					Target:   ins.host + ".",
				}
				res.Answer = append(res.Answer, rr)
			}
		default:
			res.SetRcode(req, dns.RcodeNameError)
		}

		res.Authoritative = true
		res.RecursionAvailable = false

	respond:
		err = w.WriteMsg(res)
		if err != nil {
			log.Printf("[warning] write msg failed: %s", err)
		}

		// TODO(alx): Put logging in central place for control in different
		//						environemnts.
		log.Printf("query: %s %s -> %d\n", dns.TypeToString[q.Qtype], q.Name, len(res.Answer))
	}
}

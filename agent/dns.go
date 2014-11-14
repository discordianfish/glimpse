package main

import (
	"log"
	"strings"

	"github.com/miekg/dns"
)

func runDNS(addr, zone, domain string, store store) error {
	server := &dns.Server{
		Addr: addr,
		Net:  "udp",
	}

	dns.HandleFunc(".", dnsHandler(store, zone, domain))

	return server.ListenAndServe()
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
		// TODO(ts): Only set authoritative if name can be parsed
		res.Authoritative = true
		res.RecursionAvailable = false

		switch q.Qtype {
		case dns.TypeSRV:
			addr := q.Name

			// Trim domain if present as it is not relevant for the extraction from the
			// service address.
			if strings.Contains(addr, domain) {
				addr = strings.TrimSuffix(addr, domain+".")
			}

			// Trim trailing dot of fqdn
			addr = strings.TrimSuffix(addr, ".")

			srv, err := infoFromAddr(addr, zone)
			if err != nil {
				log.Printf("err: extract lookup '%s': %s", q.Name, err)
				res.SetRcode(req, dns.RcodeServerFailure)
				break
			}

			nodes, err := store.getInstances(srv)
			if err != nil {
				log.Fatalf("consul lookup failed: %s", err)
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
			log.Fatalf("response failed: %s", err)
		}

		// TODO(alx): Put logging in central place for control in different
		//						environemnts.
		log.Printf("query: %s %s -> %d\n", dns.TypeToString[q.Qtype], q.Name, len(res.Answer))
	}
}

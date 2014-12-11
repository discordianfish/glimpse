package main

import (
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
)

func dnsHandler(store store, zone, domain string) dns.HandlerFunc {
	return func(w dns.ResponseWriter, req *dns.Msg) {
		var (
			addr      string
			err       error
			instances instances
			srv       info
			q         dns.Question

			res = &dns.Msg{}
		)

		res.SetReply(req)

		if len(req.Question) == 0 {
			res.SetRcode(req, dns.RcodeFormatError)
			goto respond
		}

		// http://maradns.samiam.org/multiple.qdcount.html
		if len(req.Question) > 1 {
			res.SetRcode(req, dns.RcodeNotImplemented)
			goto respond
		}

		q = req.Question[0]

		if !strings.HasSuffix(q.Name, "."+domain) {
			log.Printf("[warning] dns - unknown domain %s (authoritative zone: %s)", q.Name, domain)
			res.SetRcode(req, dns.RcodeNameError)
			goto respond
		}

		// Trim domain as it is not relevant for the extraction from the
		// service address.
		addr = strings.TrimSuffix(q.Name, "."+domain)

		srv, err = infoFromAddr(addr)
		if err != nil {
			log.Printf("[warning] dns - address parsing failed '%s': %s", q.Name, err)
			res.SetRcode(req, dns.RcodeNameError)
			goto respond
		}

		instances, err = store.getInstances(srv)
		if err != nil {
			// TODO(ts): Maybe return NoError for registered service without
			//           instances.
			if isNoInstances(err) {
				res.SetRcode(req, dns.RcodeNameError)
				goto respond
			}

			log.Printf("[error] store - lookup failed '%s': %s", q.Name, err)
			res.SetRcode(req, dns.RcodeServerFailure)
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
					Target:   dns.Fqdn(ins.host),
				}
				res.Answer = append(res.Answer, rr)
			}
		default:
			res.SetRcode(req, dns.RcodeNotImplemented)
			goto respond
		}

		res.Authoritative = true
		res.RecursionAvailable = false

	respond:
		err = w.WriteMsg(res)
		if err != nil {
			log.Printf("[error] dns - write msg failed: %s", err)
		}

		reqInfo := dns.TypeToString[q.Qtype] + " " + q.Name
		if q.Qtype == dns.TypeNone {
			reqInfo = "<empty>"
		}
		// TODO(alx): Put logging in central place for control in different
		//            environemnts.
		log.Printf("[info] dns - request: %s response: %s (%d rrs)",
			reqInfo, dns.RcodeToString[res.Rcode], len(res.Answer))
	}
}

// TODO(alx): Settle on naming for handlers acting as middleware.
func protocolHandler(maxAnswers int, next dns.Handler) dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		var (
			buf      = &bufferedWriter{w, nil}
			_, isUDP = w.RemoteAddr().(*net.UDPAddr)
		)

		next.ServeDNS(buf, r)

		if isUDP && len(buf.msg.Answer) > maxAnswers {
			buf.msg.Answer = buf.msg.Answer[:maxAnswers]
			buf.msg.Truncated = true
		}

		w.WriteMsg(buf.msg)
	})
}

type bufferedWriter struct {
	dns.ResponseWriter

	msg *dns.Msg
}

func (w *bufferedWriter) WriteMsg(m *dns.Msg) error {
	w.msg = m

	return nil
}

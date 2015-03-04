package main

import (
	"log"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const (
	defaultTTL = 5 * time.Second
)

func dnsHandler(store store, zone string, domains []string) dns.HandlerFunc {
	return func(w dns.ResponseWriter, req *dns.Msg) {
		var (
			addr      string
			domain    string
			err       error
			instances instances
			srv       info
			q         dns.Question

			res = &dns.Msg{}
		)

		res.SetReply(req)
		res.Compress = true

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

		for _, d := range domains {
			if strings.HasSuffix(q.Name, d) {
				domain = d
				break
			}
		}

		if domain == "" {
			res.SetRcode(req, dns.RcodeNameError)
			goto respond
		}

		res.Authoritative = true
		res.RecursionAvailable = false

		// Trim domain as it is not longer relevant for further processing.
		addr = ""
		if i := strings.LastIndex(q.Name, "."+domain); i > 0 {
			addr = q.Name[:i]
		}

		switch q.Qtype {
		case dns.TypeA, dns.TypeSRV:
			srv, err = infoFromAddr(addr)
			if err != nil {
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

				log.Printf("store - lookup failed '%s': %s", q.Name, err)
				res.SetRcode(req, dns.RcodeServerFailure)
				goto respond
			}

			for _, i := range instances {
				res.Answer = append(res.Answer, newRR(q, i))
			}
		case dns.TypeNS:
			if addr != "" {
				if err := validateZone(addr); err != nil {
					res.SetRcode(req, dns.RcodeNameError)
					goto respond
				}
			}

			instances, err = store.getServers(addr)
			if err != nil && !isNoInstances(err) {
				log.Printf("store - lookup failed '%s': %s", q.Name, err)
				res.SetRcode(req, dns.RcodeServerFailure)
				goto respond
			}

			for _, i := range instances {
				res.Answer = append(res.Answer, newRR(q, i))
			}
		default:
			res.SetRcode(req, dns.RcodeNotImplemented)
			goto respond
		}

	respond:
		err = w.WriteMsg(res)
		if err != nil {
			log.Printf("DNS - write msg failed: %s", err)
		}

		reqInfo := dns.TypeToString[q.Qtype] + " " + q.Name
		if q.Qtype == dns.TypeNone {
			reqInfo = "<empty>"
		}
		// TODO(alx): Put logging in central place for control in different
		//            environemnts.
		log.Printf("DNS - request: %s response: %s (%d rrs)",
			reqInfo, dns.RcodeToString[res.Rcode], len(res.Answer))
	}
}

func newRR(q dns.Question, i instance) dns.RR {
	hdr := dns.RR_Header{
		Name:   q.Name,
		Rrtype: q.Qtype,
		Class:  dns.ClassINET,
		Ttl:    uint32(defaultTTL.Seconds()),
	}

	switch q.Qtype {
	case dns.TypeA:
		return &dns.A{
			Hdr: hdr,
			A:   i.ip,
		}
	case dns.TypeSRV:
		return &dns.SRV{
			Hdr:      hdr,
			Priority: 0,
			Weight:   0,
			Port:     i.port,
			Target:   dns.Fqdn(i.host),
		}
	case dns.TypeNS:
		return &dns.NS{
			Hdr: hdr,
			Ns:  dns.Fqdn(i.host),
		}
	default:
		panic("unreachable")
	}
}

// TODO(alx): Settle on naming for handlers acting as middleware.
func protocolHandler(maxAnswers int, next dns.Handler) dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		next.ServeDNS(&truncatingWriter{w, maxAnswers}, r)
	})
}

type truncatingWriter struct {
	dns.ResponseWriter
	maxAnswers int
}

func (w *truncatingWriter) WriteMsg(m *dns.Msg) error {
	_, isUDP := w.RemoteAddr().(*net.UDPAddr)

	if isUDP && len(m.Answer) > w.maxAnswers {
		m.Answer = m.Answer[:w.maxAnswers]
		m.Truncated = true
	}

	return w.ResponseWriter.WriteMsg(m)
}

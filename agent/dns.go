package main

import (
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

			res = newResponse(req)
		)

		if len(req.Question) == 0 {
			res.SetRcode(req, dns.RcodeFormatError)
			w.WriteMsg(res)
			return
		}

		// http://maradns.samiam.org/multiple.qdcount.html
		if len(req.Question) > 1 {
			res.SetRcode(req, dns.RcodeNotImplemented)
			w.WriteMsg(res)
			return
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
			w.WriteMsg(res)
			return
		}

		res.Authoritative = true

		// Trim domain as it is not longer relevant for further processing.
		addr = ""
		if i := strings.LastIndex(q.Name, "."+domain); i > 0 {
			addr = q.Name[:i]
		}

		if !validDomain(addr) {
			res.SetRcode(req, dns.RcodeNameError)
			w.WriteMsg(res)
			return
		}

		switch q.Qtype {
		case dns.TypeA, dns.TypeSRV:
			srv, err = infoFromAddr(addr)
			if err != nil {
				res.SetRcode(req, dns.RcodeNameError)
				break
			}

			instances, err = store.getInstances(srv)
			if err != nil {
				// TODO(ts): Maybe return NoError for registered service without
				//           instances.
				if isNoInstances(err) {
					res.SetRcode(req, dns.RcodeNameError)
					break
				}

				res.SetRcode(req, dns.RcodeServerFailure)
				break
			}

			for _, i := range instances {
				res.Answer = append(res.Answer, newRR(q, i))
			}
		case dns.TypeNS:
			if addr != "" {
				if err := validateZone(addr); err != nil {
					res.SetRcode(req, dns.RcodeNameError)
					break
				}
			}

			instances, err = store.getServers(addr)
			if err != nil && !isNoInstances(err) {
				res.SetRcode(req, dns.RcodeServerFailure)
				break
			}

			for _, i := range instances {
				res.Answer = append(res.Answer, newRR(q, i))
			}
		}

		w.WriteMsg(res)
	}
}

func newResponse(req *dns.Msg) *dns.Msg {
	res := &dns.Msg{}
	res.SetReply(req)
	res.Compress = true
	res.RecursionAvailable = false
	return res
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

func validDomain(q string) bool {
	if q == "" {
		return true
	}
	fields := strings.Split(q, ".")
	if len(fields) == 1 || len(fields) == 5 {
		if err := validateZone(fields[len(fields)-1]); err == nil {
			return true
		}
	}
	return false
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

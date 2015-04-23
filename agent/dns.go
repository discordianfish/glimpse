package main

import (
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const (
	// defaultTTL specifies the time in seconds a response can be cached.
	defaultTTL uint32 = 5

	// defaultInvalidTTL specifies the time in seconds a NXDOMAIN response for
	// a question format not supported by glimpse-agent can be cached following
	// https://tools.ietf.org/html/rfc2308#section-5.
	defaultInvalidTTL uint32 = 86400
)

var (
	serviceQuestionRE = regexp.MustCompile(`^([[:alnum:]\-]+\.){4}[[:alnum:]]{2}$`)
	serverQuestionRE  = regexp.MustCompile(`^([[:alnum:]]{2})?$`)
)

type dnsHandler struct {
	store  store
	domain string
}

func newDNSHandler(store store, domain string) *dnsHandler {
	return &dnsHandler{
		store:  store,
		domain: domain,
	}
}

func (h *dnsHandler) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	var res = newResponse(req)

	// http://maradns.samiam.org/multiple.qdcount.html
	if len(req.Question) > 1 {
		res.Rcode = dns.RcodeNotImplemented
		w.WriteMsg(res)
		return
	}

	q := req.Question[0]

	if !strings.HasSuffix(q.Name, h.domain) {
		res.Rcode = dns.RcodeNameError
		w.WriteMsg(res)
		return
	}

	res.Authoritative = true

	// Trim domain as it is not longer relevant for further processing.
	name := ""
	if i := strings.LastIndex(q.Name, "."+h.domain); i > 0 {
		name = q.Name[:i]
	}

	switch {
	case serviceQuestionRE.MatchString(name):
		h.serviceResponse(name, q, res)
	case serverQuestionRE.MatchString(name):
		h.serverResponse(name, q, res)
	default:
		res.Rcode = dns.RcodeNameError
		res.Extra = append(res.Extra, newSOA(q, h.domain, defaultInvalidTTL))
	}

	w.WriteMsg(res)
}

func (h *dnsHandler) serviceResponse(name string, q dns.Question, res *dns.Msg) {
	if q.Qtype != dns.TypeA && q.Qtype != dns.TypeSRV {
		return
	}

	srv, err := infoFromAddr(name)
	if err != nil {
		res.Rcode = dns.RcodeNameError
		return
	}

	instances, err := h.store.getInstances(srv)
	if err != nil {
		// TODO(ts): Maybe return NoError for registered service without
		//           instances.
		if isNoInstances(err) {
			res.Rcode = dns.RcodeNameError
			return
		}

		res.Rcode = dns.RcodeServerFailure
		return
	}

	for _, i := range instances {
		res.Answer = append(res.Answer, newRR(q, i))
	}
}

func (h *dnsHandler) serverResponse(name string, q dns.Question, res *dns.Msg) {
	if q.Qtype != dns.TypeA && q.Qtype != dns.TypeNS {
		return
	}

	if name != "" {
		if err := validateZone(name); err != nil {
			res.Rcode = dns.RcodeNameError
			return
		}
	}

	servers, err := h.store.getServers(name)
	if err != nil && !isNoInstances(err) {
		res.Rcode = dns.RcodeServerFailure
		return
	}

	for _, s := range servers {
		res.Answer = append(res.Answer, newRR(q, s))
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
		Ttl:    defaultTTL,
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

func newSOA(q dns.Question, domain string, ttl uint32) dns.RR {
	return &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   q.Name,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ns:      "ns0." + domain,
		Mbox:    "hostmaster." + domain,
		Serial:  uint32(time.Now().Unix()),
		Refresh: 3600,
		Retry:   600,
		Expire:  86400,
		Minttl:  defaultTTL,
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

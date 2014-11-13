package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/armon/consul-api"
	"github.com/miekg/dns"
)

const (
	defaultDomain = "srv.glimpse.io"
	defaultZone   = "gg"

	routeLookup = `/lookup/{name:[a-z0-9\-\.]+}`
)

var (
	rDomain = regexp.MustCompile(`^[a-z0-9]+([\-\.]{1}[a-z0-9]+)*\.[a-z]{2,6}$`)
	rField  = regexp.MustCompile(`^[[:alnum:]\-]+$`)
	rZone   = regexp.MustCompile(`^[[:alnum:]]{2}$`)
)

type srvInfo struct {
	env     string
	job     string
	product string
	service string
	zone    string
}

func main() {
	var (
		consulAddr = flag.String("consul.addr", "127.0.0.1:8500", "consul lookup address")
		srvDomain  = flag.String("srv.domain", defaultDomain, "srv lookup domain")
		srvZone    = flag.String("srv.zone", defaultZone, "srv lookup zone")
		udpAddr    = flag.String("udp.addr", ":5959", "udp address to bind to")
	)
	flag.Parse()
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(os.Stdout)

	consul, err := consulapi.NewClient(&consulapi.Config{
		Address:    *consulAddr,
		Datacenter: *srvZone,
	})
	if err != nil {
		log.Fatalf("consul connection failed: %s", err)
	}

	server := &dns.Server{
		Addr: *udpAddr,
		Net:  "udp",
	}

	dns.HandleFunc(".", handleRequest(consul, *srvZone, *srvDomain))

	log.Printf("glimpse-agent started on %s\n", *udpAddr)
	log.Fatalf("dns failed: %s", server.ListenAndServe())
}

func handleRequest(consul *consulapi.Client, zone, domain string) dns.HandlerFunc {
	return func(w dns.ResponseWriter, req *dns.Msg) {
		var (
			q   = req.Question[0]
			res = &dns.Msg{}
		)

		if len(req.Question) > 1 {
			log.Printf("warn: question > 1: %+v\n", req.Question)

			for _, q := range req.Question {
				log.Printf("warn: %s %s %s\n", dns.TypeToString[q.Qtype], dns.ClassToString[q.Qclass], q.Name)
			}
		}

		res.SetReply(req)
		res.Authoritative = true
		res.RecursionAvailable = false

		switch q.Qtype {
		case dns.TypeSRV:
			srv, err := extractSrvInfo(strings.TrimSuffix(q.Name, "."), zone, domain)
			if err != nil {
				log.Printf("err: extract lookup '%s': %s", q.Name, err)
				res.SetRcode(req, dns.RcodeServerFailure)
				break
			}

			nodes, err := consulLookup(srv, consul)
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
					Port:     uint16(n.ServicePort),
					Target:   n.Node + ".",
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

		log.Printf("query: %s %s -> %d\n", dns.TypeToString[q.Qtype], q.Name, len(res.Answer))
	}
}

func consulLookup(srv srvInfo, consul *consulapi.Client) ([]*consulapi.CatalogService, error) {
	var (
		envTag     = fmt.Sprintf("glimpse:env=%s", srv.env)
		jobTag     = fmt.Sprintf("glimpse:job=%s", srv.job)
		serviceTag = fmt.Sprintf("glimpse:service=%s", srv.service)
		options    = &consulapi.QueryOptions{
			AllowStale: true,
			Datacenter: srv.zone,
		}

		nodes = []*consulapi.CatalogService{}
	)

	catalog := consul.Catalog()
	allNodes, meta, err := catalog.Service(srv.product, jobTag, options)
	if err != nil {
		return nil, err
	}

	log.Printf(
		"consul lookup of %s.*.%s took %dns\n",
		srv.product,
		srv.job,
		meta.RequestTime.Nanoseconds(),
	)

	for _, node := range allNodes {
		var (
			isEnv     bool
			isService bool
		)

		for _, tag := range node.ServiceTags {
			if tag == envTag {
				isEnv = true
			}
			if tag == serviceTag {
				isService = true
			}
		}

		if isEnv && isService {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

func extractSrvInfo(name, zone, domain string) (srvInfo, error) {
	var (
		fields = strings.SplitN(name, ".", 6)
		l      = len(fields)
	)

	switch {
	case l < 4: // Misses some information
		return srvInfo{}, fmt.Errorf("the name is invalid")
	case l == 5: // zone is present: service.job.env.product.zone
		zone = fields[4]
	case l == 6:
		domain = fields[5]
		zone = fields[4]
	}

	var (
		product = fields[3]
		env     = fields[2]
		job     = fields[1]
		service = fields[0]
	)

	if !rDomain.MatchString(domain) {
		return srvInfo{}, fmt.Errorf("domain %q is invalid", domain)
	}
	if !rZone.MatchString(zone) {
		return srvInfo{}, fmt.Errorf("zone %q is invalid", zone)
	}
	if !rField.MatchString(product) {
		return srvInfo{}, fmt.Errorf("product %q is invalid", product)
	}
	if !rField.MatchString(env) {
		return srvInfo{}, fmt.Errorf("env %q is invalid", env)
	}
	if !rField.MatchString(job) {
		return srvInfo{}, fmt.Errorf("job %q is invalid", job)
	}
	if !rField.MatchString(service) {
		return srvInfo{}, fmt.Errorf("service %q is invalid", service)
	}

	return srvInfo{
		env:     env,
		job:     job,
		product: product,
		service: service,
		zone:    zone,
	}, nil
}

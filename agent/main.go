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

	client, err := consulapi.NewClient(&consulapi.Config{
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

	store := &consulStore{
		client: client,
	}

	dns.HandleFunc(".", dnsHandler(store, *srvZone, *srvDomain))

	log.Printf("glimpse-agent started on %s\n", *udpAddr)
	log.Fatalf("dns failed: %s", server.ListenAndServe())
}

func extractSrvInfo(name, zone, domain string) (info, error) {
	var (
		fields = strings.SplitN(name, ".", 6)
		l      = len(fields)
	)

	switch {
	case l < 4: // Misses some information
		return info{}, fmt.Errorf("the name is invalid")
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
		return info{}, fmt.Errorf("domain %q is invalid", domain)
	}
	if !rZone.MatchString(zone) {
		return info{}, fmt.Errorf("zone %q is invalid", zone)
	}
	if !rField.MatchString(product) {
		return info{}, fmt.Errorf("product %q is invalid", product)
	}
	if !rField.MatchString(env) {
		return info{}, fmt.Errorf("env %q is invalid", env)
	}
	if !rField.MatchString(job) {
		return info{}, fmt.Errorf("job %q is invalid", job)
	}
	if !rField.MatchString(service) {
		return info{}, fmt.Errorf("service %q is invalid", service)
	}

	return info{
		env:     env,
		job:     job,
		product: product,
		service: service,
		zone:    zone,
	}, nil
}

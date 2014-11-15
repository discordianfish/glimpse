package main

import (
	"flag"
	"log"
	"os"
	"regexp"

	"github.com/armon/consul-api"
	"github.com/miekg/dns"
)

const (
	defaultDNSZone = "srv.glimpse.io"
	defaultSrvZone = "gg"

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
		dnsAddr    = flag.String("dns.addr", ":5959", "DNS address to bind to")
		dnsZone    = flag.String("dns.zone", defaultDNSZone, "DNS zone")
		srvZone    = flag.String("srv.zone", defaultSrvZone, "srv zone")
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

	store := &consulStore{
		client: client,
	}

	server := &dns.Server{
		Addr: *dnsAddr,
		Net:  "udp",
	}

	dns.HandleFunc(".", dnsHandler(store, *srvZone, *dnsZone))

	log.Printf("glimpse-agent starting on %s\n", *dnsAddr)
	log.Fatalf("DNS failed: %s", server.ListenAndServe())
}

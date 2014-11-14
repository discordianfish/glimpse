package main

import (
	"flag"
	"log"
	"os"
	"regexp"

	"github.com/armon/consul-api"
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
		dnsAddr    = flag.String("dns.addr", ":5959", "DNS address to bind to")
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

	log.Printf("glimpse-agent starting on %s\n", *dnsAddr)
	log.Fatalf("dns failed: %s", runDNS(*dnsAddr, *srvZone, *srvDomain, store))
}

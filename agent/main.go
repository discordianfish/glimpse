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

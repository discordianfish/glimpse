package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/armon/consul-api"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultDNSZone    = "srv.glimpse.io"
	defaultSrvZone    = "gg"
	defaultMaxAnswers = 43 // TODO(alx): Find sane defaults.

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
		maxAnswers = flag.Int("dns.udp.maxanswers", defaultMaxAnswers, "DNS maximum answers returned via UDP")
		dnsZone    = flag.String("dns.zone", defaultDNSZone, "DNS zone")
		srvZone    = flag.String("srv.zone", defaultSrvZone, "srv zone")
		httpAddr   = flag.String("http.addr", ":5960", "HTTP address to bind to")
	)
	flag.Parse()
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(os.Stdout)
	log.SetPrefix("glimpse-agent ")

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

	dnsMux := dns.NewServeMux()
	dnsMux.Handle(
		".",
		dnsMetricsHandler(
			protocolHandler(
				*maxAnswers,
				dnsHandler(
					store,
					*srvZone,
					dns.Fqdn(*dnsZone),
				),
			),
		),
	)

	http.Handle("/metrics", prometheus.Handler())

	errc := make(chan error, 1)

	go runDNSServer(&dns.Server{
		Addr:    *dnsAddr,
		Handler: dnsMux,
		Net:     "tcp",
	}, errc)
	go runDNSServer(&dns.Server{
		Addr:    *dnsAddr,
		Handler: dnsMux,
		Net:     "udp",
	}, errc)
	go func(addr string, errc chan<- error) {
		log.Printf("HTTP listening on %s\n", addr)
		errc <- http.ListenAndServe(addr, nil)
	}(*httpAddr, errc)

	select {
	case err := <-errc:
		log.Fatalf("%s", err)
	}
}

func runDNSServer(server *dns.Server, errc chan error) {
	log.Printf("DNS/%s listening on %s\n", server.Net, server.Addr)
	errc <- server.ListenAndServe()
}

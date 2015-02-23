package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/hashicorp/consul/api"
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
	// ldflag
	version = "0.0.0.dev"

	rDNSZone = regexp.MustCompile(`^[a-z0-9]+([\-\.]{1}[a-z0-9]+)*\.[a-z]{2,6}$`)
)

func main() {
	var (
		consulAddr = flag.String("consul.addr", "127.0.0.1:8500", "consul lookup address")
		consulInfo = flag.String("consul.info", "consul info", "info command")
		dnsAddr    = flag.String("dns.addr", ":5959", "DNS address to bind to")
		maxAnswers = flag.Int("dns.udp.maxanswers", defaultMaxAnswers, "DNS maximum answers returned via UDP")
		dnsZone    = flag.String("dns.zone", defaultDNSZone, "DNS zone")
		srvZone    = flag.String("srv.zone", defaultSrvZone, "srv zone")
		httpAddr   = flag.String("http.addr", ":5960", "HTTP address to bind to")
	)
	flag.Parse()

	log.SetFlags(log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	log.SetPrefix("glimpse-agent ")

	if !rDNSZone.MatchString(*dnsZone) {
		log.Fatalf("invalid DNS zone: %s", *dnsZone)
	}

	log.Printf("[info] glimpse-agent starting. v%s", version)
	client, err := api.NewClient(&api.Config{
		Address:    *consulAddr,
		Datacenter: *srvZone,
	})
	if err != nil {
		log.Fatalf("consul connection failed: %s", err)
	}

	var (
		errc  = make(chan error, 1)
		store = newMetricsStore(newConsulStore(client))
	)

	http.Handle("/metrics", prometheus.Handler())

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
		log.Printf("[info] HTTP listening on %s\n", addr)
		err := http.ListenAndServe(addr, nil)
		errc <- fmt.Errorf("[error] HTTP - server failed: %s", err)
	}(*httpAddr, errc)
	go func(errc chan<- error) { errc <- interrupt() }(errc)

	if *consulInfo != "" {
		go registerConsulCollector(*consulInfo)
	}

	log.Fatalln(<-errc)
}

func interrupt() error {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	return fmt.Errorf("[info] got signal: %s. Good bye.", <-c)
}

func runDNSServer(server *dns.Server, errc chan error) {
	log.Printf("[info] DNS/%s listening on %s\n", server.Net, server.Addr)
	err := server.ListenAndServe()
	errc <- fmt.Errorf("[error] DNS/%s - server failed: %s", server.Net, err)
}

func registerConsulCollector(consulInfo string) {
	c := newConsulCollector(consulInfo)

	for {
		if err := prometheus.Register(c); err != nil {
			log.Printf("[error] prometheus - could not register collector"+
				" (-consul.info=%s)", consulInfo)
			<-time.After(1 * time.Second)
			continue
		}

		break
	}
}

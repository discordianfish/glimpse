package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
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
	defaultDNSZone    = "srv.glimpse.io."
	defaultSrvZone    = "gg"
	defaultMaxAnswers = 43 // TODO(alx): Find sane defaults.

	routeLookup = `/lookup/{name:[a-z0-9\-\.]+}`
)

var (
	// Set during buildtime.
	version = "0.0.0.dev"

	rDNSZone = regexp.MustCompile(`^[a-z0-9]+([\-\.]{1}[a-z0-9]+)*\.[a-z]{2,6}\.?$`)
	logger   = log.New(os.Stdout, "glimpse-agent ", log.Lmicroseconds)
)

func main() {
	var (
		dnsZones dnsZones

		consulAddr = flag.String("consul.addr", "127.0.0.1:8500", "consul lookup address")
		consulInfo = flag.String("consul.info", "consul info", "info command")
		dnsAddr    = flag.String("dns.addr", ":5959", "DNS address to bind to")
		srvZone    = flag.String("srv.zone", defaultSrvZone, "srv zone")
		httpAddr   = flag.String("http.addr", ":5960", "HTTP address to bind to")
		maxAnswers = flag.Int(
			"dns.udp.maxanswers",
			defaultMaxAnswers,
			"DNS maximum answers returned via UDP",
		)
	)
	flag.Var(&dnsZones, "dns.zone", "DNS zone")
	flag.Parse()

	if len(dnsZones) == 0 {
		dnsZones = append(dnsZones, defaultDNSZone)
	}

	log.Printf("glimpse-agent starting. v%s", version)
	client, err := api.NewClient(&api.Config{
		Address:    *consulAddr,
		Datacenter: *srvZone,
	})
	if err != nil {
		logger.Fatalf("consul connection failed: %s", err)
	}

	var (
		errc  = make(chan error, 1)
		store = newLoggingStore(
			logger,
			newMetricsStore(
				newConsulStore(
					client,
				),
			),
		)
	)

	http.Handle("/metrics", prometheus.Handler())

	dnsMux := dns.NewServeMux()
	dnsMux.Handle(
		".",
		dnsLoggingHandler(
			logger,
			dnsMetricsHandler(
				protocolHandler(
					*maxAnswers,
					dnsHandler(
						store,
						*srvZone,
						dnsZones,
					),
				),
			),
		),
	)

	// DNS TCP server
	go runDNSServer(&dns.Server{
		Addr:    *dnsAddr,
		Handler: dnsMux,
		Net:     "tcp",
	}, errc)
	// DNS UDP server
	go runDNSServer(&dns.Server{
		Addr:    *dnsAddr,
		Handler: dnsMux,
		Net:     "udp",
	}, errc)

	// HTTP server
	go func(addr string, errc chan<- error) {
		logger.Printf("HTTP listening on %s\n", addr)
		errc <- fmt.Errorf(
			"[error] HTTP - server failed: %s",
			http.ListenAndServe(addr, nil),
		)
	}(*httpAddr, errc)

	// Signal handling
	go func(errc chan<- error) { errc <- interrupt() }(errc)

	if *consulInfo != "" {
		go registerConsulCollector(*consulInfo)
	}

	logger.Fatalln(<-errc)
}

func interrupt() error {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	return fmt.Errorf("[info] got signal: %s. Good bye", <-c)
}

func runDNSServer(server *dns.Server, errc chan error) {
	logger.Printf("DNS/%s listening on %s\n", server.Net, server.Addr)
	errc <- fmt.Errorf(
		"[error] DNS/%s - server failed: %s", server.Net,
		server.ListenAndServe(),
	)
}

func registerConsulCollector(consulInfo string) {
	c := newConsulCollector(consulInfo)

	for {
		if err := prometheus.Register(c); err != nil {
			logger.Printf(
				"prometheus - could not register collector (-consul.info=%s)",
				consulInfo,
			)
			<-time.After(1 * time.Second)
			continue
		}

		break
	}
}

type dnsZones []string

func (z *dnsZones) String() string {
	return fmt.Sprintf("%s", *z)
}

func (z *dnsZones) Set(value string) error {
	if !rDNSZone.MatchString(value) {
		return fmt.Errorf("invalid DNS zone: %s", value)
	}
	*z = append(*z, dns.Fqdn(value))
	return nil
}

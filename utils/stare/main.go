package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

func main() {
	var (
		addr    = flag.String("addr", "localhost:53", "DNS server address")
		qps     = flag.Int("qps", 10, "DNS queries per second")
		query   = flag.String("q", "", "test query")
		logging = flag.Bool("log", false, "log errors")
		net     = flag.String("net", "udp", "DNS network protocol")
	)
	flag.Parse()

	if *query == "" {
		fmt.Fprint(os.Stderr, "flag must be provided: -q\n")
		flag.Usage()
		os.Exit(1)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	client := newClient(*addr, *net)

	var requests, errors uint64
	go func() {
		for range time.Tick(time.Second / time.Duration(*qps)) {
			go func(q string) {
				if err := client.query(q); err != nil {
					if *logging {
						log.Print(err)
					}
					atomic.AddUint64(&errors, 1)
				}
			}(*query)
			atomic.AddUint64(&requests, 1)
		}
	}()

	var lastr, laste uint64
	for range time.Tick(time.Second) {
		r, e := atomic.LoadUint64(&requests), atomic.LoadUint64(&errors)
		fmt.Printf("\r%s [%d r/s, %d err/s]", *query, r-lastr, e-laste)
		lastr, laste = r, e
	}
}

type client struct {
	addr, net string
	*dns.Client
}

func newClient(addr, net string) client {
	return client{
		addr:   addr,
		Client: &dns.Client{Net: net},
	}
}

func (c client) query(q string) error {
	m := &dns.Msg{}
	m.SetQuestion(q, dns.TypeSRV)
	r, _, err := c.Client.Exchange(m, c.addr)
	if err != nil {
		return fmt.Errorf("failed %s: %s", q, err)
	}
	if r.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("failed %s: unexpected %s", q, dns.RcodeToString[r.Rcode])
	}
	return nil
}

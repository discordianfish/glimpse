package main

import (
	"flag"
	"log"
	"runtime"

	"github.com/miekg/dns"
)

func main() {
	var (
		a = flag.String("addr", "ns0.sd.int.s-cloud.net:53", "glimpse agent addr")
		c = flag.Int("c", 50, "DNS query concurrency")
	)
	flag.Parse()

	tests := []string{
		"telemetry.kraken-api.prod.timeline.dd.sd.int.s-cloud.net.",
		"telemetry.agent.prod.glimpse.dd.sd.int.s-cloud.net.",
		"http.api.prod.v2.dd.sd.int.s-cloud.net.",
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	d := newClient(*a)
	p := make(chan struct{}, *c)
	i := 0
	for {
		for _, q := range tests {
			p <- struct{}{}
			go d.query(q, p)
			i++

			if i%*c == 0 {
				log.Printf("%d requests", *c)
			}
		}
	}
}

type client struct {
	addr string

	*dns.Client
}

func newClient(addr string) client {
	return client{
		addr:   addr,
		Client: &dns.Client{Net: "tcp"},
	}
}

func (c client) query(q string, p chan struct{}) {
	defer func() { <-p }()

	m := &dns.Msg{}
	m.SetQuestion(q, dns.TypeSRV)

	r, _, err := c.Client.Exchange(m, c.addr)
	if err != nil {
		log.Printf("failed %s: %s", q, err)
		return
	}
	if dns.RcodeSuccess != r.Rcode {
		log.Printf(
			"failed %s: unexpected %s",
			q, dns.RcodeToString[r.Rcode])
		return
	}
	//log.Printf("success %s: %d answers", q, len(r.Answer))
}

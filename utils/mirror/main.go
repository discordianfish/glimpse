package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ActiveState/tail"
	"github.com/miekg/dns"
)

func main() {
	var (
		srcLog      = flag.String("src.log", "/etc/unbound/unbound.log", "unbound's log file location")
		srcDomain   = flag.String("src.domain", "src.example.com.", "source domain to copy requests from")
		dstDomain   = flag.String("dst.domain", "dst.example.com.", "destination domain to send requests to")
		dstServer   = flag.String("dst.server", "localhost:53", "destination server to send requests to")
		dstProtocol = flag.String("dst.protocol", "udp", "destination protocol")
	)
	flag.Parse()

	if *srcLog == "" {
		fmt.Fprint(os.Stderr, "flag must be provided: -src.log\n")
		flag.Usage()
		os.Exit(1)
	}

	t, err := tail.TailFile(*srcLog, tail.Config{
		Location: &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END},
		ReOpen:   true,
		Follow:   true,
	})
	if err != nil {
		log.Fatalf("error reading file %s: %s", *srcLog, err)
	}

	var (
		client = newClient(*dstServer, *dstProtocol)
		ticker = time.NewTicker(time.Second)
		pool   = make(chan struct{}, 50)

		count     int
		lastCount int
		position  string
	)
	for {
		select {
		case line := <-t.Lines:
			if line.Err != nil {
				continue
			}

			fields := strings.Fields(line.Text)
			if len(fields) != 7 || fields[3] != "resolving" {
				continue
			}

			sname, qtype := fields[4], fields[5]
			if !strings.HasSuffix(sname, *srcDomain) {
				continue
			}

			dname := strings.Replace(sname, *srcDomain, *dstDomain, -1)
			pool <- struct{}{}
			go func(q, qtype string) {
				if err := client.query(dname, qtype); err != nil {
					//log.Println(err)
				}
				<-pool
			}(dname, qtype)
			count++
			position = fields[0]
		case <-ticker.C:
			ts := "unknown"
			if len(position) >= 12 {
				i, err := strconv.ParseInt(position[1:len(position)-1], 10, 64)
				if err != nil {
					panic(err)
				}
				ts = time.Unix(i, 0).Format("2006/01/02 15:04:05")
			}
			log.Printf("read %s: mirrored %d r/s", ts, count-lastCount)
			lastCount = count
		}
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

func (c client) query(q, qtype string) error {
	m := &dns.Msg{}
	m.SetQuestion(q, dns.StringToType[qtype])

	r, _, err := c.Client.Exchange(m, c.addr)
	if err != nil {
		return fmt.Errorf("failed %s %s: %s", q, qtype, err)
	}

	switch r.Rcode {
	case dns.RcodeSuccess, dns.RcodeNameError, dns.RcodeNotImplemented:
	default:
		return fmt.Errorf("failed %s %s: unexpected %s",
			q, qtype, dns.RcodeToString[r.Rcode])
	}
	return nil
}

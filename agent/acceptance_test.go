// +build acceptance

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/miekg/dns"
)

const (
	// cmd control
	cmdTimeout = 5 * time.Second

	// consul-agent
	advertise = "1.2.3.4"
	consulBin = ".deps/consul"
	nodeName  = "hokuspokus"

	// glimpse-agent
	dnsAddr       = "127.0.0.1:5555"
	dnsZone       = "test.glimpse.io"
	dnsMaxAnswers = 3
	httpAddr      = "127.0.0.1:5556"
	srvZone       = "cz"
)

// test data
var (
	testCase0 = testCase{
		instances: 1,
		failing:   1,
		port:      8000,
		provider:  "harpoon",
		srvAddr:   "http.stream.prod.goku",
	}
	testCase1 = testCase{
		instances: dnsMaxAnswers + (rand.Intn(dnsMaxAnswers) + 1),
		port:      9000,
		provider:  "bazooka",
		srvAddr:   "http.walker.staging.roshi",
	}
)

func TestAgent(t *testing.T) {
	tests := []struct {
		query   string
		qtype   uint16
		net     string
		rcode   int
		answers []string
	}{
		{
			query:   fqdn(testCase0.srvAddr, srvZone, dnsZone),
			qtype:   dns.TypeA,
			net:     "udp",
			answers: []string{advertise},
		},
		{
			query:   fqdn(testCase0.srvAddr, srvZone, dnsZone),
			qtype:   dns.TypeA,
			net:     "tcp",
			answers: []string{advertise},
		},
		{
			query:   fqdn(testCase0.srvAddr, srvZone, dnsZone),
			qtype:   dns.TypeSRV,
			net:     "udp",
			answers: []string{fmt.Sprintf("%s.:%d", nodeName, testCase0.port)},
		},
		{
			query:   fqdn(testCase0.srvAddr, srvZone, dnsZone),
			qtype:   dns.TypeSRV,
			net:     "tcp",
			answers: []string{fmt.Sprintf("%s.:%d", nodeName, testCase0.port)},
		},
		{
			query:   fqdn(srvZone, dnsZone),
			qtype:   dns.TypeNS,
			net:     "udp",
			answers: []string{fqdn("ns0", srvZone, dnsZone)},
		},
		{
			query:   fqdn(srvZone, dnsZone),
			qtype:   dns.TypeNS,
			net:     "tcp",
			answers: []string{fqdn("ns0", srvZone, dnsZone)},
		},
		{
			query:   fqdn(dnsZone),
			qtype:   dns.TypeNS,
			net:     "udp",
			answers: []string{fqdn("ns0", dnsZone)},
		},
		{
			query:   fqdn("ns0", dnsZone),
			qtype:   dns.TypeA,
			net:     "udp",
			answers: []string{advertise},
		},
		{
			query:   fqdn("ns0", srvZone, dnsZone),
			qtype:   dns.TypeA,
			net:     "udp",
			answers: []string{advertise},
		},
		{
			query: fqdn(testCase0.srvAddr),
			qtype: dns.TypeSRV,
			net:   "udp",
			rcode: dns.RcodeNameError,
		},
		{
			query: fqdn(testCase0.srvAddr, srvZone),
			qtype: dns.TypeSRV,
			net:   "udp",
			rcode: dns.RcodeNameError,
		},
		{
			query: fqdn(testCase0.srvAddr, dnsZone),
			qtype: dns.TypeSRV,
			net:   "udp",
			rcode: dns.RcodeNameError,
		},
		{
			query: fqdn(testCase0.srvAddr, srvZone, "example.domain"),
			qtype: dns.TypeSRV,
			net:   "udp",
			rcode: dns.RcodeNameError,
		},
		{
			query: fqdn("foo.bar.baz.qux", srvZone, dnsZone),
			qtype: dns.TypeSRV,
			net:   "udp",
			rcode: dns.RcodeNameError,
		},
		{
			query: fqdn(testCase0.srvAddr, "ff", dnsZone),
			qtype: dns.TypeA,
			net:   "tcp",
			rcode: dns.RcodeNameError,
		},
	}

	consul, err := runConsul()
	if err != nil {
		t.Fatalf("consul failed: %s", err)
	}
	defer consul.terminate()

	go func() {
		err := <-consul.errc
		if err != nil {
			t.Fatal(err)
		}
	}()

	agent, err := runAgent()
	if err != nil {
		t.Fatalf("agent failed: %s", err)
	}
	defer agent.terminate()

	go func() {
		err := <-agent.errc
		if err != nil {
			t.Fatal(err)
		}
	}()

	for _, test := range tests {
		res, err := query(test.query, test.qtype, test.net)
		if err != nil {
			t.Fatalf("DNS lookup failed: %s", err)
		}
		if want, got := test.rcode, res.Rcode; want != got {
			m := dns.RcodeToString
			t.Fatalf("%s want rcode '%s', got '%s'", test.query, m[want], m[got])
		}
		if want, got := len(test.answers), len(res.Answer); want != got {
			t.Fatalf("%s want %d answers, got %d", test.query, want, got)
		}
		for i, answer := range res.Answer {
			if want, got := test.query, answer.Header().Name; want != got {
				t.Fatalf("%s want '%s', got '%s'", test.query, want, got)
			}

			switch test.qtype {
			case dns.TypeA:
				a, ok := answer.(*dns.A)
				if !ok {
					t.Fatalf("failed to extract A type")
				}
				if want, got := test.answers[i], a.A.String(); want != got {
					t.Fatalf("want A %s, got %s", want, got)
				}
			case dns.TypeSRV:
				srv, ok := answer.(*dns.SRV)
				if !ok {
					t.Fatalf("failed to extract SRV type")
				}
				s := strings.Split(test.answers[i], ":")
				if want, got := s[0], srv.Target; want != got {
					t.Fatalf("want target %s, got %s", want, got)
				}
				p, _ := strconv.Atoi(s[1])
				if want, got := uint16(p), srv.Port; want != got {
					t.Fatalf("want port %d, got %d", want, got)
				}
			case dns.TypeNS:
				ns, ok := answer.(*dns.NS)
				if !ok {
					t.Fatalf("failed to extract NS type")
				}
				if want, got := test.answers[i], ns.Ns; want != got {
					t.Fatalf("want NS %s, got %s", want, got)
				}
			}
		}
	}

	// truncation
	q := fqdn(testCase1.srvAddr, srvZone, dnsZone)

	res, err := query(q, dns.TypeSRV, "udp")
	if err != nil {
		t.Fatalf("DNS/udp lookup failed: %s", err)
	}

	if want, got := true, res.Truncated; want != got {
		t.Fatalf("want msg truncated, got '%t'", got)
	}

	// metrics
	m, err := http.Get(fmt.Sprintf("http://%s/metrics", httpAddr))
	if err != nil {
		t.Errorf("HTTP metrics request failed: %s", err)
	}

	if want, got := 200, m.StatusCode; want != got {
		t.Errorf("want HTTP code %d, got %d", want, got)
	}

	defer m.Body.Close()
	body, err := ioutil.ReadAll(m.Body)
	if err != nil {
		t.Fatalf("HTTP metrics can't read body: %s", err)
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(string(body), metric) {
			t.Errorf("want %s in HTTP body:\n%s", metric, string(body))
		}
	}
}

func TestAgentMissingConsul(t *testing.T) {
	a, err := runAgent()
	if err != nil {
		t.Fatalf("want agent to run without consul-agent: %s", err)
	}
	a.terminate()
}

type testCase struct {
	failing   int
	instances int
	port      int
	provider  string
	srvAddr   string
}

func generateServicesConfig(addr, provider string, port int, isFailing bool) ([]byte, string, error) {
	info, err := infoFromAddr(addr)
	if err != nil {
		return nil, "", err
	}

	info.provider = provider

	id := fmt.Sprintf(
		"%s-%s-%s-%s-%d",
		info.product,
		info.env,
		info.job,
		info.service,
		port,
	)

	type check struct {
		Script   string `json:"script"`
		Interval string `json:"interval"`
	}

	type service struct {
		ID    string   `json:"id"`
		Name  string   `json:"name"`
		Tags  []string `json:"tags"`
		Port  int      `json:"port"`
		Check *check   `json:"check,omitempty"`
	}

	s := &service{
		ID:    id,
		Name:  info.product,
		Tags:  infoToTags(info),
		Port:  port,
		Check: nil,
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}

	if isFailing {
		s.Check = &check{
			Script:   filepath.Join(strings.TrimSuffix(wd, "agent"), "misc", "scripts", "failing"),
			Interval: "1s",
		}
	}

	b, err := json.Marshal(&struct {
		Service *service `json:"service"`
	}{
		Service: s,
	})
	if err != nil {
		return nil, "", err
	}

	return b, id, nil
}

func query(q string, t uint16, net string) (*dns.Msg, error) {
	var (
		m = &dns.Msg{}
		c = &dns.Client{
			Net: net,
		}
	)

	m.SetQuestion(q, t)

	res, _, err := c.Exchange(m, dnsAddr)
	return res, err
}

func runAgent() (*cmd, error) {
	args := []string{
		"-consul.info", consulBin + " info -rpc-addr localhost:8400",
		"-dns.addr", dnsAddr,
		"-dns.udp.maxanswers", strconv.Itoa(dnsMaxAnswers),
		"-dns.zone", dnsZone,
		"-http.addr", httpAddr,
		"-srv.zone", srvZone,
	}

	return runCmd("./glimpse-agent", args, func() bool {
		_, err := query("", dns.TypeNS, "udp")
		if err != nil {
			return false
		}
		return true
	})
}

func runConsul() (*cmd, error) {
	configDir, err := ioutil.TempDir("", "config")
	if err != nil {
		return nil, fmt.Errorf("failed to create consul data dir: %s", err)
	}
	defer os.RemoveAll(configDir)

	dataDir, err := ioutil.TempDir("", "data")
	if err != nil {
		return nil, fmt.Errorf("failed to create consul data dir: %s", err)
	}
	defer os.RemoveAll(dataDir)

	for _, c := range []testCase{
		testCase0,
		testCase1,
	} {
		for i := 0; i < c.instances; i++ {
			cfg, id, err := generateServicesConfig(fmt.Sprintf("%s.%s", c.srvAddr, srvZone), c.provider, c.port+i, false)
			if err != nil {
				return nil, fmt.Errorf("config gen failed: %s", err)
			}

			err = ioutil.WriteFile(path.Join(configDir, fmt.Sprintf("%s.json", id)), cfg, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to write config: %s", err)
			}
		}

		for i := c.instances; i < c.instances+c.failing; i++ {
			cfg, id, err := generateServicesConfig(fmt.Sprintf("%s.%s", c.srvAddr, srvZone), c.provider, c.port+i, true)
			if err != nil {
				return nil, fmt.Errorf("config gen failed: %s", err)
			}

			err = ioutil.WriteFile(path.Join(configDir, fmt.Sprintf("%s.json", id)), cfg, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to write config: %s", err)
			}
		}
	}

	args := []string{
		"agent",
		"-server",
		"-advertise", advertise,
		"-bootstrap-expect", "1",
		"-dc", srvZone,
		"-node", nodeName,
		"-config-dir", configDir,
		"-data-dir", dataDir,
	}

	return runCmd(consulBin, args, func() bool {
		client, err := api.NewClient(&api.Config{
			Address:    "127.0.0.1:8500",
			Datacenter: srvZone,
			HttpClient: (&http.Client{
				Timeout: 100 * time.Millisecond,
			}),
		})
		if err != nil {
			panic(err)
		}

		is, _, err := client.Catalog().Service("goku", "", nil)
		if err != nil || len(is) < 2 {
			return false
		}

		return true
	})
}

type cmd struct {
	cmd        *exec.Cmd
	name       string
	errc       chan error
	args       []string
	stdout     []string
	stderr     []string
	terminatec chan chan error
}

func runCmd(name string, args []string, readyFn func() bool) (*cmd, error) {
	c := exec.Command(name, args...)

	outPipe, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	errPipe, err := c.StderrPipe()
	if err != nil {
		return nil, err
	}

	var (
		stdoutc = make(chan string)
		stderrc = make(chan string)
		errc    = make(chan error, 3)
	)

	go readLines(outPipe, stdoutc, errc)
	go readLines(errPipe, stderrc, errc)

	err = c.Start()
	if err != nil {
		return nil, err
	}

	go func(cmd *exec.Cmd, errc chan error) {
		errc <- cmd.Wait()
	}(c, errc)

	cmd := &cmd{
		cmd:        c,
		name:       name,
		args:       args,
		stdout:     []string{},
		stderr:     []string{},
		errc:       make(chan error, 1),
		terminatec: make(chan chan error),
	}

	readyc := make(chan error)

	go cmd.run(stdoutc, stderrc, errc)
	go readyCheck(readyc, readyFn)

	return cmd, <-readyc
}

func (c *cmd) run(stdoutc, stderrc <-chan string, errc chan error) {
	for {
		select {
		case line := <-stdoutc:
			c.stdout = append(c.stdout, line)
		case line := <-stderrc:
			c.stderr = append(c.stderr, line)
		case ec := <-c.terminatec:
			ec <- syscall.Kill(c.cmd.Process.Pid, syscall.SIGTERM)
			return
		case err := <-errc:
			if err != nil {
				err = fmt.Errorf(
					"%s failed: %s\nstdout:\n%s\nstderr:\n%s",
					c.name,
					err,
					strings.Join(c.stdout, "\n"),
					strings.Join(c.stderr, "\n"),
				)
			}
			c.errc <- err
			return
		}
	}
}

func (c *cmd) terminate() error {
	errc := make(chan error)

	c.terminatec <- errc
	return <-errc
}

func readyCheck(readyc chan error, readyFn func() bool) {
	start := time.Now()

	for !readyFn() {
		if time.Since(start) > cmdTimeout {
			readyc <- fmt.Errorf("time out")
			return
		}

		time.Sleep(100 * time.Millisecond)
	}

	close(readyc)
}

func readLines(pipe io.ReadCloser, outc chan string, errc chan error) {
	reader := bufio.NewReader(pipe)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				continue
			}
			if _, ok := err.(*os.PathError); ok {
				return
			}
			errc <- err
		}
		outc <- string(line)
	}
}

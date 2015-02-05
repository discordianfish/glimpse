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

	var (
		q string
	)

	// success - SRV
	q = dns.Fqdn(fmt.Sprintf("%s.%s.%s", testCase0.srvAddr, srvZone, dnsZone))

	res, err := query(q, dns.TypeSRV, "udp")
	if err != nil {
		t.Fatalf("DNS lookup failed: %s", err)
	}

	want, got := dns.RcodeToString[dns.RcodeSuccess], dns.RcodeToString[res.Rcode]
	if want != got {
		t.Fatalf("%s: want rcode '%s', got '%s'", q, want, got)
	}

	if want, got := testCase0.instances, len(res.Answer); want != got {
		t.Fatalf("want %d DNS result, got %d", want, got)
	}

	if want, got := q, res.Answer[0].Header().Name; want != got {
		t.Fatalf("want '%s', got '%s'", want, got)
	}

	srv, ok := res.Answer[0].(*dns.SRV)
	if !ok {
		t.Fatalf("failed to extract SRV type")
	}

	if want, got := dns.Fqdn(nodeName), srv.Target; want != got {
		t.Fatalf("want target %s, got %s", want, got)
	}

	if want, got := uint16(testCase0.port), srv.Port; want != got {
		t.Fatalf("want port %d, got %d", want, got)
	}

	// success - A
	q = dns.Fqdn(fmt.Sprintf("%s.%s.%s", testCase0.srvAddr, srvZone, dnsZone))

	res, err = query(q, dns.TypeA, "udp")
	if err != nil {
		t.Fatalf("DNS lookup failed: %s", err)
	}

	want, got = dns.RcodeToString[dns.RcodeSuccess], dns.RcodeToString[res.Rcode]
	if want != got {
		t.Fatalf("want rcode '%s', got '%s'", want, got)
	}

	if want, got := testCase0.instances, len(res.Answer); want != got {
		t.Fatalf("want %d DNS result, got %d", want, got)
	}

	if want, got := q, res.Answer[0].Header().Name; want != got {
		t.Fatalf("want '%s', got '%s'", want, got)
	}

	a, ok := res.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("failed to extract A type")
	}

	if want, got := advertise, a.A.String(); want != got {
		t.Fatalf("want A %s, got %s", want, got)
	}

	// success - NS
	q = dns.Fqdn(fmt.Sprintf("%s.%s", srvZone, dnsZone))

	res, err = query(q, dns.TypeNS, "udp")
	if err != nil {
		t.Fatalf("DNS lookup failed: %s", err)
	}

	want, got = dns.RcodeToString[dns.RcodeSuccess], dns.RcodeToString[res.Rcode]
	if want != got {
		t.Fatalf("want rcode '%s', got '%s'", want, got)
	}

	if want, got := 1, len(res.Answer); want != got {
		t.Fatalf("want %d DNS result, got %d", want, got)
	}

	if want, got := q, res.Answer[0].Header().Name; want != got {
		t.Fatalf("want '%s', got '%s'", want, got)
	}

	ns, ok := res.Answer[0].(*dns.NS)
	if !ok {
		t.Fatalf("failed to extract NS type")
	}

	if want, got := dns.Fqdn(nodeName), ns.Ns; want != got {
		t.Fatalf("want NS %s, got %s", want, got)
	}

	// fail - non-existent DNS zone
	for _, q := range []string{
		dns.Fqdn(testCase0.srvAddr),
		dns.Fqdn(fmt.Sprintf("%s.%s", testCase0.srvAddr, srvZone)),
		dns.Fqdn(fmt.Sprintf("%s.%s", testCase0.srvAddr, dnsZone)),
		dns.Fqdn(fmt.Sprintf("%s.%s.example.domain", testCase0.srvAddr, srvZone)),
		dns.Fqdn(fmt.Sprintf("foo.bar.baz.qux.%s.%s", srvZone, dnsZone)),
	} {
		res, err := query(q, dns.TypeSRV, "udp")
		if err != nil {
			t.Fatalf("DNS lookup failed: %s", err)
		}

		want, got := dns.RcodeToString[dns.RcodeNameError], dns.RcodeToString[res.Rcode]
		if want != got {
			t.Fatalf("%s: want rcode '%s', got '%s'", q, want, got)
		}
	}

	// success - TCP
	q = dns.Fqdn(fmt.Sprintf("%s.%s.%s", testCase1.srvAddr, srvZone, dnsZone))

	res, err = query(q, dns.TypeSRV, "udp")
	if err != nil {
		t.Fatalf("DNS/udp lookup failed: %s", err)
	}

	if want, got := true, res.Truncated; want != got {
		t.Fatalf("want msg truncated, got '%t'", got)
	}

	res, err = query(q, dns.TypeSRV, "tcp")
	if err != nil {
		t.Fatalf("DNS/tcp lookup failed: %s", err)
	}

	want, got = dns.RcodeToString[dns.RcodeSuccess], dns.RcodeToString[res.Rcode]
	if want != got {
		t.Fatalf("%s: want rcode '%s', got '%s'", q, want, got)
	}

	if want, got := testCase1.instances, len(res.Answer); want != got {
		t.Fatalf("want %d DNS result, got %d", want, got)
	}

	// success - metrics
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

	for _, metric := range []string{
		`glimpse_agent_dns_request_duration_microseconds_count`,
		`glimpse_agent_consul_request_duration_microseconds_count`,
		`glimpse_agent_consul_responses`,
		`process_open_fds`,
		`consul_process_open_fds`,
	} {
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
		"-consul.bin", consulBin,
		"-dns.addr", dnsAddr,
		"-dns.udp.maxanswers", strconv.Itoa(dnsMaxAnswers),
		"-dns.zone", dnsZone,
		"-http.addr", httpAddr,
		"-srv.zone", srvZone,
	}

	cmd, err := runCmd("./glimpse-agent", args, "udp listening", 1)
	if err != nil {
		return nil, err
	}

	select {
	case <-cmd.readyc:
		return cmd, nil
	case err := <-cmd.errc:
		return nil, err
	}
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

	cmd, err := runCmd(consulBin, args, "is now critical", 5)
	if err != nil {
		return nil, err
	}

	select {
	case <-cmd.readyc:
		return cmd, nil
	case err := <-cmd.errc:
		return nil, err
	}
}

type cmd struct {
	cmd              *exec.Cmd
	name             string
	check            string
	checkOccurrences int
	errc             chan error
	readyc           chan struct{}
	args             []string
	stdout           []string
	stderr           []string
	terminating      bool
}

func runCmd(name string, args []string, check string, occur int) (*cmd, error) {
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
		cmd:              c,
		name:             name,
		args:             args,
		check:            check,
		checkOccurrences: occur,
		errc:             make(chan error, 1),
		readyc:           make(chan struct{}),
		stdout:           []string{},
		stderr:           []string{},
	}

	go cmd.run(stdoutc, stderrc, errc)

	return cmd, nil
}

func (c *cmd) run(stdoutc, stderrc <-chan string, errc <-chan error) {
	var (
		count = 0
		ready = false
	)

	for {
		select {
		case line := <-stdoutc:
			c.stdout = append(c.stdout, line)

			if strings.Contains(line, c.check) {
				count++
			}
			if count == c.checkOccurrences {
				c.readyc <- struct{}{}
				ready = true
			}
		case line := <-stderrc:
			c.stderr = append(c.stderr, line)
		case err := <-errc:
			if c.terminating {
				return
			}

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
		case <-time.After(cmdTimeout):
			if c.terminating {
				return
			}
			if ready {
				continue
			}
			c.errc <- fmt.Errorf(
				"%s timed out\nstdout:\n%s\nstderr:\n%s",
				c.name,
				strings.Join(c.stdout, "\n"),
				strings.Join(c.stderr, "\n"),
			)
			return
		}
	}
}

func (c *cmd) terminate() error {
	c.terminating = true
	err := syscall.Kill(c.cmd.Process.Pid, syscall.SIGTERM)
	if err != nil {
		return err
	}
	return nil
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
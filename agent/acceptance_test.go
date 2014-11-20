// +build acceptance

package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
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

	// glimpse-agent
	addr       = "127.0.0.1:5959"
	dnsZone    = "test.glimpse.io"
	srvZone    = "cz"
	maxAnswers = 3

	// consul-agent
	advertise = "1.2.3.4"
	nodeName  = "hokuspokus"
)

// test data
var (
	testCase0 = testCase{
		instances: 1,
		port:      8000,
		provider:  "harpoon",
		srvAddr:   "http.stream.prod.goku",
	}
	testCase1 = testCase{
		instances: maxAnswers * rand.Intn(maxAnswers),
		port:      9000,
		provider:  "bazooka",
		srvAddr:   "http.walker.staging.roshi",
	}
)

func TestAll(t *testing.T) {
	configDir, err := ioutil.TempDir("", "config")
	if err != nil {
		t.Fatalf("failed to create consul data dir: %s", err)
	}
	defer os.RemoveAll(configDir)

	dataDir, err := ioutil.TempDir("", "data")
	if err != nil {
		t.Fatalf("failed to create consul data dir: %s", err)
	}
	defer os.RemoveAll(dataDir)

	for _, c := range []testCase{
		testCase0,
		testCase1,
	} {
		for i := 0; i < c.instances; i++ {
			cfg, id, err := generateConfig(fmt.Sprintf("%s.%s", c.srvAddr, srvZone), c.provider, c.port+i)
			if err != nil {
				t.Fatalf("config gen failed: %s", err)
			}

			err = ioutil.WriteFile(path.Join(configDir, fmt.Sprintf("%s.json", id)), cfg, 0644)
			if err != nil {
				t.Fatalf("failed to write config: %s", err)
			}
		}
	}

	consul, err := runConsul(configDir, dataDir)
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
		q   string
		hdr *dns.RR_Header
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

	hdr = res.Answer[0].Header()

	if want, got := q, hdr.Name; want != got {
		t.Fatalf("want '%s', got '%s'", want, got)
	}

	srv, ok := res.Answer[0].(*dns.SRV)
	if !ok {
		t.Fatalf("failed to extract SRV type")
	}

	if want, got := fmt.Sprintf("%s.", nodeName), srv.Target; want != got {
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

	hdr = res.Answer[0].Header()

	if want, got := q, hdr.Name; want != got {
		t.Fatalf("want '%s', got '%s'", want, got)
	}

	a, ok := res.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("failed to extract A type")
	}

	if want, got := advertise, a.A.String(); want != got {
		t.Fatalf("want A %s, got %s", want, got)
	}

	// fail - non-existent DNS zone
	for _, q := range []string{
		dns.Fqdn(testCase0.srvAddr),
		dns.Fqdn(fmt.Sprintf("%s.%s", testCase0.srvAddr, srvZone)),
		dns.Fqdn(fmt.Sprintf("%s.%s", testCase0.srvAddr, dnsZone)),
		dns.Fqdn(fmt.Sprintf("%s.%s.example.domain", testCase0.srvAddr, srvZone)),
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
}

type testCase struct {
	instances int
	port      int
	provider  string
	srvAddr   string
}

func generateConfig(addr, provider string, port int) ([]byte, string, error) {
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

	return []byte(fmt.Sprintf(
		`
{
	"service": {
		"id": "%s",
		"name": "%s",
		"tags": [
			"glimpse:provider=%s",
			"glimpse:product=%s",
			"glimpse:env=%s",
			"glimpse:job=%s",
			"glimpse:service=%s"
		],
		"port": %d
	}
}
`,
		id,
		info.product,
		info.provider,
		info.product,
		info.env,
		info.job,
		info.service,
		port,
	)), id, nil
}

func query(q string, t uint16, net string) (*dns.Msg, error) {
	var (
		m = &dns.Msg{}
		c = &dns.Client{
			Net: net,
		}
	)

	m.SetQuestion(q, t)

	res, _, err := c.Exchange(m, addr)
	return res, err
}

func runAgent() (*cmd, error) {
	args := []string{
		"-dns.addr", addr,
		"-dns.udp.maxanswers", strconv.Itoa(maxAnswers),
		"-dns.zone", dnsZone,
		"-srv.zone", srvZone,
	}

	cmd, err := runCmd("./glimpse-agent", args, "udp listening")
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

func runConsul(configDir, dataDir string) (*cmd, error) {
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

	cmd, err := runCmd(".deps/consul", args, "Synced service 'goku")
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
	cmd    *exec.Cmd
	name   string
	check  string
	errc   chan error
	readyc chan struct{}
	args   []string
	stdout []string
	stderr []string
}

func runCmd(name string, args []string, check string) (*cmd, error) {
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
		cmd:    c,
		name:   name,
		args:   args,
		check:  check,
		errc:   make(chan error, 1),
		readyc: make(chan struct{}),
		stdout: []string{},
		stderr: []string{},
	}

	go cmd.run(stdoutc, stderrc, errc)

	return cmd, nil
}

func (c *cmd) run(stdoutc chan string, stderrc chan string, errc chan error) {
	ready := false

	for {
		select {
		case line := <-stdoutc:
			c.stdout = append(c.stdout, line)

			if strings.Contains(line, c.check) {
				c.readyc <- struct{}{}
				ready = true
			}
		case line := <-stderrc:
			c.stderr = append(c.stderr, line)
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
		case <-time.After(cmdTimeout):
			if ready {
				continue
			}
			c.errc <- fmt.Errorf(
				"% timed out:\nstdout:\n%s\nstderr:\n%s",
				c.name,
				strings.Join(c.stdout, "\n"),
				strings.Join(c.stderr, "\n"),
			)
			return
		}
	}
}

func (c *cmd) terminate() error {
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

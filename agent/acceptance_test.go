// +build acceptance

package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/miekg/dns"
)

var config = []byte(`
{
	"service": {
		"id": "goku-stream-8080",
		"name": "goku",
		"tags": [
			"glimpse:provider=harpoon",
			"glimpse:product=goku",
			"glimpse:env=prod",
			"glimpse:job=stream",
			"glimpse:service=http"
		],
		"port": 8080
	}
}
`)

func TestAll(t *testing.T) {
	// TODO(alx): Account for missing env variable.
	buildDir := os.Getenv("BUILD_DIR")

	configDir, err := ioutil.TempDir(buildDir, "config")
	if err != nil {
		t.Fatalf("failed to create consul data dir: %s", err)
	}
	defer os.RemoveAll(configDir)

	dataDir, err := ioutil.TempDir(buildDir, "data")
	if err != nil {
		t.Fatalf("failed to create consul data dir: %s", err)
	}
	defer os.RemoveAll(dataDir)

	err = ioutil.WriteFile(path.Join(configDir, "test.json"), config, 0644)
	if err != nil {
		t.Fatalf("failed to write config: %s", err)
	}

	consul, err := runConsul(configDir, dataDir)
	if err != nil {
		t.Fatalf("consul failed: %s", err)
	}
	defer terminateCommand(consul)

	agent, err := runAgent()
	if err != nil {
		t.Fatalf("agent failed: %s", err)
	}
	defer terminateCommand(agent)

	c := &dns.Client{}
	m := &dns.Msg{}

	m.SetQuestion("http.stream.prod.goku.", dns.TypeSRV)

	res, _, err := c.Exchange(m, "127.0.0.1:5959")
	if err != nil {
		t.Fatalf("DNS lookup failed: %s", err)
	}

	if len(res.Answer) != 1 {
		t.Fatalf("expected 1 DNS result, got %d", len(res.Answer))
	}

	var (
		hdr      = res.Answer[0].Header()
		expected = "http.stream.prod.goku."
		got      = hdr.Name
	)

	if expected != got {
		t.Fatalf("expected '%s', got '%s'", expected, got)
	}

	expected = dns.TypeToString[dns.TypeSRV]
	got = dns.TypeToString[hdr.Rrtype]

	if expected != got {
		t.Fatalf("expected '%s', got '%s'", expected, got)
	}
}

func runAgent() (*exec.Cmd, error) {
	args := []string{
		"-srv.zone", "cz",
		"-udp.addr", ":5959",
	}

	return runCommand(".deps/glimpse-agent", args, "glimpse-agent started")
}

func runConsul(configDir, dataDir string) (*exec.Cmd, error) {
	args := []string{
		"agent",
		"-server",
		"-bootstrap-expect", "1",
		"-dc", "cz",
		"-config-dir", configDir,
		"-data-dir", dataDir,
	}

	return runCommand(".deps/consul", args, "Synced service 'goku")
}

func runCommand(name string, args []string, success string) (*exec.Cmd, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	var (
		linec = make(chan string)
		errc  = make(chan error)
	)

	// TODO(alx): Better coordination of routines and proper shutdown.
	go func(out io.ReadCloser, linec chan string, errc chan error) {
		reader := bufio.NewReader(out)
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
			linec <- string(line)
		}
	}(out, linec, errc)

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	go func(cmd *exec.Cmd, errc chan error) {
		errc <- cmd.Wait()
	}(cmd, errc)

	var lastLine string

	for {
		select {
		case line := <-linec:
			lastLine = line

			if strings.Contains(line, success) {
				return cmd, nil
			}
		case err := <-errc:
			if err != nil {
				return nil, fmt.Errorf("%s: %s", err, lastLine)
			}
		case <-time.After(5 * time.Second):
			return nil, fmt.Errorf("% startup timed out: %s", name, lastLine)
		}
	}
}

func terminateCommand(cmd *exec.Cmd) {
	err := syscall.Kill(cmd.Process.Pid, syscall.SIGTERM)
	if err != nil {
		panic(err)
	}
}

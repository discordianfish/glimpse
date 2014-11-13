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

	"github.com/armon/consul-api"
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
	// defer os.RemoveAll(configDir)

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

	defer func(consul *exec.Cmd) {
		err := syscall.Kill(consul.Process.Pid, syscall.SIGTERM)
		if err != nil {
			panic(err)
		}
	}(consul)

	client, err := consulapi.NewClient(consulapi.DefaultConfig())
	if err != nil {
		t.Fatalf("consul connection failed: %s", err)
	}

	services, err := client.Agent().Services()
	if err != nil {
		t.Fatalf("services failed: %s", err)
	}

	_, ok := services["goku-stream-8080"]
	if !ok {
		t.Fatal("service not present in agent")
	}
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

	cmd := exec.Command(".deps/consul", args...)
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

			if strings.Contains(line, "Synced service 'goku") {
				return cmd, nil
			}
		case err := <-errc:
			if err != nil {
				return nil, fmt.Errorf("%s: %s", err, lastLine)
			}
		case <-time.After(5 * time.Second):
			return nil, fmt.Errorf("consul startup timed out: %s", lastLine)
		}
	}
}

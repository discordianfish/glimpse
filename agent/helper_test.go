package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/miekg/dns"
)

// brokenStore implements the glimpse.store interface.
type brokenStore struct{}

func (s *brokenStore) getInstances(srv info) (instances, error) {
	return nil, newError(errConsulAPI, "could not get instances")
}

func (s *brokenStore) getServers(zone string) (instances, error) {
	return nil, newError(errConsulAPI, "could not get servers")
}

// testStore implements the glimpse.store interface.
type testStore struct {
	instances map[info]instances
	servers   map[string]instances
}

func (s *testStore) getInstances(srv info) (instances, error) {
	r, ok := s.instances[srv]
	if !ok {
		return nil, newError(errNoInstances, "")
	}
	return r, nil
}

func (s *testStore) getServers(zone string) (instances, error) {
	if zone != "" {
		return s.servers[zone], nil
	}

	r := instances{}
	for _, s := range s.servers {
		r = append(r, s...)
	}
	return r, nil
}

// testWriter implements the dns.ResponseWriter interface.
type testWriter struct {
	msg        *dns.Msg
	remoteAddr net.Addr
}

func (w *testWriter) WriteMsg(m *dns.Msg) error {
	w.msg = m
	return nil
}

func (w *testWriter) LocalAddr() net.Addr         { return nil }
func (w *testWriter) RemoteAddr() net.Addr        { return w.remoteAddr }
func (w *testWriter) Write(s []byte) (int, error) { return 0, nil }
func (w *testWriter) Close() error                { return nil }
func (w *testWriter) TsigStatus() error           { return nil }
func (w *testWriter) TsigTimersOnly(b bool)       {}
func (w *testWriter) Hijack()                     {}

// helpers
func fqdn(s ...string) string {
	return dns.Fqdn(strings.Join(s, "."))
}

func createServiceEntry(
	i info,
	port int,
	host, ip string,
	checks []*api.HealthCheck,
) *api.ServiceEntry {
	return &api.ServiceEntry{
		Node: &api.Node{
			Node:    host,
			Address: ip,
		},
		Service: &api.AgentService{
			ID:      fmt.Sprintf("%s-%s-%d", i.product, i.job, port),
			Service: i.product,
			Tags:    infoToTags(i),
			Port:    port,
		},
		Checks: checks,
	}
}

func generateInstances(addrs ...string) (instances, error) {
	ins := instances{}

	for _, addr := range addrs {
		i, err := infoFromAddr(addr)
		if err != nil {
			return nil, err
		}

		ins = append(ins, generateInstancesFromInfo(i)...)
	}

	return ins, nil
}

func generateInstancesFromInfo(i info) instances {
	var (
		n   = rand.Intn(10) + 1
		ins = make(instances, n)
	)

	for j := 0; j < n; j++ {
		ins[j] = instance{
			host: "suppenkasper",
			ip:   net.ParseIP("1.2.3.4"),
			port: uint16(20000 + j),
		}
	}

	return ins
}

func setupStubConsul(
	result interface{},
	t *testing.T,
) (*api.Client, *httptest.Server) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				err := json.NewEncoder(w).Encode(result)
				if err != nil {
					t.Fatalf("encoding response failed: %s", err)
				}
			},
		),
	)

	url, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("server url parse failed: %s", err)
	}

	client, err := api.NewClient(&api.Config{
		Address:    url.Host,
		Datacenter: defaultSrvZone,
	})
	if err != nil {
		t.Fatalf("consul setup failed: %s", err)
	}

	return client, server
}

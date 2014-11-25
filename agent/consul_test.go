package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/armon/consul-api"
	consul "github.com/hashicorp/consul/consul/structs"
)

type test struct {
	want  int
	input []*consulapi.CatalogService
}

func TestConsulGetInstances(t *testing.T) {
	i, err := infoFromAddr("http.walker.qa.roshi.gg")
	if err != nil {
		t.Fatalf("info extraction failed: %s", err)
	}
	result := []*consulapi.ServiceEntry{
		createServiceEntry(i, 8080, "host00.gg.local", "10.2.3.4", nil),
	}

	client, server := setupStubConsul(result, t)
	defer server.Close()

	store := newConsulStore(client)

	is, err := store.getInstances(i)
	if err != nil {
		t.Fatalf("getInstances failed: %s", err)
	}
	if want, got := len(result), len(is); want != got {
		t.Errorf("want %d instances, got %d", want, got)
	}
}

func TestConsulGetInstancesEmptyResult(t *testing.T) {
	client, server := setupStubConsul([]*consulapi.CatalogService{}, t)
	defer server.Close()

	store := newConsulStore(client)

	i, err := infoFromAddr("predict.future.experimental.oracle.gg")
	if err != nil {
		t.Fatalf("info extraction failed: %s", err)
	}

	_, err = store.getInstances(i)

	if !isNoInstances(err) {
		t.Errorf("want %s, got %s", errNoInstances, err)
	}
}

func TestConsulGetInstancesFailingCheck(t *testing.T) {
	i, err := infoFromAddr("xmpp.chat.prod.fire.gg")
	if err != nil {
		t.Fatalf("info extraction failed: %s", err)
	}

	var (
		host   = "host02.gg.local"
		ip     = "10.3.4.5"
		result = []*consulapi.ServiceEntry{
			createServiceEntry(i, 9090, host, ip, nil),
			createServiceEntry(i, 9091, host, ip, nil),
			createServiceEntry(i, 9092, host, ip, []*consulapi.HealthCheck{
				&consulapi.HealthCheck{
					Status: consul.HealthCritical,
				},
			}),
		}
	)

	client, server := setupStubConsul(result, t)
	defer server.Close()

	store := newConsulStore(client)

	is, err := store.getInstances(i)
	if err != nil {
		t.Fatalf("getInstances failed: %s", err)
	}
	if want, got := len(result)-1, len(is); want != got {
		t.Errorf("want %d instances, got %d", want, got)
	}
}

func TestConsulGetInstancesInvalidIP(t *testing.T) {
	i, err := infoFromAddr("prometheus.walker.qa.roshi.gg")
	if err != nil {
		t.Fatalf("info extraction failed: %s", err)
	}
	result := []*consulapi.ServiceEntry{
		createServiceEntry(i, 8081, "host01.gg.local", "3.2.1", nil),
	}

	client, server := setupStubConsul(result, t)
	defer server.Close()

	store := newConsulStore(client)

	_, err = store.getInstances(i)
	if !isInvalidIP(err) {
		t.Fatalf("want %s, got %s", errInvalidIP, err)
	}
}

func TestConsulGetInstancesNoConsul(t *testing.T) {
	client, err := consulapi.NewClient(&consulapi.Config{
		Address:    "1.2.3.4",
		Datacenter: defaultSrvZone,
		HttpClient: &http.Client{
			Timeout: time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("consul setup failed: %s", err)
	}

	store := newConsulStore(client)

	i, err := infoFromAddr("amqp.broker.qa.solution.gg")
	if err != nil {
		t.Fatalf("info extraction failed: %s", err)
	}

	_, err = store.getInstances(i)

	if !isConsulAPI(err) {
		t.Fatalf("want %s, got %s", errConsulAPI, err)
	}
}

// TODO(alx): Test services with non-matching env/service, hence filtering in getInstances.

func createServiceEntry(
	i info,
	port int,
	host, ip string,
	checks []*consulapi.HealthCheck,
) *consulapi.ServiceEntry {
	return &consulapi.ServiceEntry{
		Node: &consulapi.Node{
			Node:    host,
			Address: ip,
		},
		Service: &consulapi.AgentService{
			ID:      fmt.Sprintf("%s-%s-%d", i.product, i.job, port),
			Service: i.product,
			Tags:    infoToTags(i),
			Port:    port,
		},
		Checks: checks,
	}
}

func setupStubConsul(
	result interface{},
	t *testing.T,
) (*consulapi.Client, *httptest.Server) {
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

	client, err := consulapi.NewClient(&consulapi.Config{
		Address:    url.Host,
		Datacenter: defaultSrvZone,
	})
	if err != nil {
		t.Fatalf("consul setup failed: %s", err)
	}

	return client, server
}

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/armon/consul-api"
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
		&consulapi.ServiceEntry{
			Node: &consulapi.Node{
				Node:    "host00.gg.local",
				Address: "10.2.3.4",
			},
			Service: &consulapi.AgentService{
				ID:      "roshi-walker-8080",
				Service: i.product,
				Tags:    infoToTags(i),
				Port:    8080,
			},
		},
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

func TestConsulGetinstancesInvalidIP(t *testing.T) {
	i, err := infoFromAddr("prometheus.walker.qa.roshi.gg")
	if err != nil {
		t.Fatalf("info extraction failed: %s", err)
	}
	result := []*consulapi.ServiceEntry{
		&consulapi.ServiceEntry{
			Node: &consulapi.Node{
				Node:    "host01.gg.local",
				Address: "3.2.1",
			},
			Service: &consulapi.AgentService{
				ID:      "roshi-walker-8081",
				Service: i.product,
				Tags:    infoToTags(i),
				Port:    8081,
			},
		},
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

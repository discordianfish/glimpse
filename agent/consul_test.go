package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
)

type test struct {
	want  int
	input []*api.CatalogService
}

func TestConsulGetInstances(t *testing.T) {
	i, err := infoFromAddr("http.walker.qa.roshi.gg")
	if err != nil {
		t.Fatalf("info extraction failed: %s", err)
	}
	result := []*api.ServiceEntry{
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
	client, server := setupStubConsul([]*api.CatalogService{}, t)
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

func TestConsulGetInstancesInvalidIP(t *testing.T) {
	i, err := infoFromAddr("prometheus.walker.qa.roshi.gg")
	if err != nil {
		t.Fatalf("info extraction failed: %s", err)
	}
	result := []*api.ServiceEntry{
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
	client, err := api.NewClient(&api.Config{
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

func TestConsulGetServers(t *testing.T) {
	result := []*api.AgentMember{
		&api.AgentMember{Name: "foo.aa"},
		&api.AgentMember{Name: "bar.aa"},
		&api.AgentMember{Name: "baz.bb"},
		&api.AgentMember{Name: "qux.bc"},
	}

	client, server := setupStubConsul(result, t)
	defer server.Close()

	store := newConsulStore(client)

	for _, test := range []struct {
		zone string
		want []string
	}{
		{zone: "aa", want: []string{"foo", "bar"}},
		{zone: "bb", want: []string{"baz"}},
		{zone: "bc", want: []string{"qux"}},
		{zone: "dd", want: []string{}},
		{zone: "", want: []string{"foo", "bar", "baz", "qux"}},
	} {
		s, err := store.getServers(test.zone)
		if err != nil {
			t.Fatalf("getServers failed: %s", err)
		}
		if want, got := len(test.want), len(s); want != got {
			t.Fatalf("want %d servers, got %d", want, got)
		}
		for i, w := range test.want {
			if want, got := w, s[i].host; want != got {
				t.Errorf("want host %s, got %s", want, got)
			}
		}
	}
}

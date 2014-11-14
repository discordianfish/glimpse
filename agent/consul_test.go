package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/armon/consul-api"
)

type test struct {
	expected int
	input    []*consulapi.CatalogService
}

func TestConsulGetInstances(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(http.NotFound))
	defer srv.Close()

	url, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("server url parse failed: %s", err)
	}

	client, err := consulapi.NewClient(&consulapi.Config{
		Address:    url.Host,
		Datacenter: defaultZone,
	})
	if err != nil {
		t.Fatalf("consul connection failed: %s", err)
	}

	info := srvInfo{
		env:     "qa",
		job:     "walker",
		product: "roshi",
		service: "http",
	}
	testSet := map[srvInfo]test{
		info: test{
			expected: 1,
			input: []*consulapi.CatalogService{
				&consulapi.CatalogService{
					Node:        "host00.gg.local",
					Address:     "10.2.3.4",
					ServiceID:   "roshi-walker-8080",
					ServiceTags: infoToTags(info),
					ServicePort: 8080,
				},
			},
		},
	}

	for info, test := range testSet {
		srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := json.NewEncoder(w).Encode(test.input)
			if err != nil {
				t.Fatalf("encoding response failed: %s", err)
			}
		})

		store := newConsulStore(client)

		is, err := store.getInstances(info)
		if err != nil {
			t.Fatalf("getInstances failed: %s", err)
		}
		if test.expected != len(is) {
			t.Errorf("expected %d instances, got %d", test.expected, len(is))
		}
	}
}

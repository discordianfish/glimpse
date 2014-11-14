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
	want  int
	input []*consulapi.CatalogService
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

	i := info{
		env:     "qa",
		job:     "walker",
		product: "roshi",
		service: "http",
	}
	// TODO(alx): Test services with non-matching env/service, hence filtering in getInstnaces.
	testSet := map[info]test{
		i: test{
			want: 1,
			input: []*consulapi.CatalogService{
				&consulapi.CatalogService{
					Node:        "host00.gg.local",
					Address:     "10.2.3.4",
					ServiceID:   "roshi-walker-8080",
					ServiceTags: infoToTags(i),
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
		if want, got := test.want, len(is); want != got {
			t.Errorf("want %d instances, got %d", want, got)
		}
	}
}

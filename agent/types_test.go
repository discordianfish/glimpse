package main

import (
	"reflect"
	"testing"
)

func TestInfoFromAddr(t *testing.T) {
	tests := map[string]info{
		"http.ent.staging.asset-hosting.ro": info{
			env:     "staging",
			job:     "ent",
			product: "asset-hosting",
			service: "http",
			zone:    "ro",
		},
		"memcached.cache.prod.search": info{
			env:     "prod",
			job:     "cache",
			product: "search",
			service: "memcached",
			zone:    defaultZone,
		},
	}

	for input, want := range tests {
		got, err := infoFromAddr(input, defaultZone)
		if err != nil {
			t.Errorf("info extraction failed '%s': %s", input, err)
			continue
		}

		if !reflect.DeepEqual(want, got) {
			t.Errorf("want %s, got %s", want, got)
		}
	}
}

func TestInforFromAddrInvalid(t *testing.T) {
	tests := []string{
		"service.job.env",                     // missing fields
		"service..env.product",                // zero-length field
		"service.job.env.product.zone",        // zone too long
		"service.job.env.product.zo.-.domain", // too many fields
		"ser/vice.job.env.product",            // invalid service
		"service.j|ob.env.product",            // invalid job
		"service.job.e^nv.product",            // invalid env
		"service.job.env.pro_duct",            // invalid product
	}

	for _, input := range tests {
		_, err := infoFromAddr(input, defaultZone)
		if err == nil {
			t.Errorf("exreaction from addr '%s' did not error", input)
		}
	}
}

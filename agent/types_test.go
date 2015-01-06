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
	}

	for input, want := range tests {
		got, err := infoFromAddr(input)
		if err != nil {
			t.Errorf("info extraction failed '%s': %s", input, err)
			continue
		}

		if !reflect.DeepEqual(want, got) {
			t.Errorf("want %s, got %s", want, got)
		}
	}
}

func TestInfoFromAddrInvalid(t *testing.T) {
	tests := []string{
		"service.job.env",                        // missing fields
		"service..env.product.ro",                // zero-length field
		"service.job.env.product.zone.ro",        // zone too long
		"service.job.env.product.zo.-.domain.ro", // too many fields
		"ser/vice.job.env.product.ro",            // invalid service
		"service.j|ob.env.product.ro",            // invalid job
		"service.job.e^nv.product.ro",            // invalid env
		"service.job.env.pro_duct.ro",            // invalid product
	}

	for _, input := range tests {
		_, err := infoFromAddr(input)
		if err == nil {
			t.Errorf("extraction from addr '%s' did not error", input)
		}
	}
}

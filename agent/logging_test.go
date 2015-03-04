package main

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLoggingStoreError(t *testing.T) {
	var (
		i = info{
			service: "xmpp",
			job:     "agent",
			env:     "qa",
			product: "mack",
			zone:    "de",
		}
		is = generateInstancesFromInfo(i)
		b  = &bytes.Buffer{}
		l  = log.New(b, "glimpse-agent ", log.Lmicroseconds)
		s  = newLoggingStore(l, &testStore{
			instances: map[info]instances{i: is},
			servers: map[string]instances{
				i.zone: is,
			},
		})
	)

	_, err := s.getInstances(i)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.getServers(i.zone)
	if err != nil {
		t.Fatal(err)
	}

	// Successfull transactions should not result in a log line.
	if want, have := 0, len(b.String()); want != have {
		t.Errorf("want %d, have %d", want, have)
	}

	_, err = s.getInstances(info{product: "nonsense"})
	if err == nil {
		t.Error("want loggingStore to pass errors")
	}

	sp := strings.Split(strings.Trim(b.String(), "\n"), " ")

	if want, have := "STORE", sp[2]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := "getInstances", sp[4]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := errToLabel[errNoInstances], sp[5]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := "...nonsense", sp[6]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	b.Reset()

	_, err = s.getServers("zz")
	if err == nil {
		t.Errorf("want loggingStore to pass errors")
	}

	sp = strings.Split(strings.Trim(b.String(), "\n"), " ")

	if want, have := "STORE", sp[2]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := "getServers", sp[4]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := errToLabel[errNoInstances], sp[5]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}

	if want, have := "zz", sp[6]; want != have {
		t.Errorf("want %s, have %s", want, have)
	}
}

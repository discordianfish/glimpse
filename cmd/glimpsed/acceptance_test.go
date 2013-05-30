package main

import (
	"fmt"
	"testing"
)

type acceptanceStore struct {
	name    string
	factory func(*testing.T) Store
}

var stores []acceptanceStore

type acceptanceTest struct {
	name string
	test func(*testing.T, Store)
}

var tests []acceptanceTest

func TestAcceptance(t *testing.T) {
	for _, store := range stores {
		t.Logf("Acceptance Tests for %+v", store.name)

		internalTests := []testing.InternalTest{}
		for _, test := range tests {
			fn := test.test
			internalTests = append(internalTests, testing.InternalTest{
				Name: fmt.Sprintf("Acceptance (%s): %s", store.name, test.name),
				F:    func(t *testing.T) { fn(t, store.factory(t)) },
			})
		}

		testing.RunTests(
			func(_, _ string) (bool, error) { return true, nil },
			internalTests,
		)
	}
}

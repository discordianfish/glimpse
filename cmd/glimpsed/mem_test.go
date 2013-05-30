package main

import (
	"testing"
)

func init() {
	stores = append(stores, acceptanceStore{
		name:    "Mem",
		factory: func(*testing.T) Store { return newMemStore() },
	})
}

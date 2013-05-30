package main

import (
	"flag"
	"testing"
)

var testDSN = flag.String("test.mysql",
	"root:@tcp(localhost:3306)/glimpse_test?timeout=30s",
	"DESTRUCTIVE MySQL connection DSN for test database")

func init() {
	stores = append(stores, acceptanceStore{
		name: "MySQL",
		factory: func(t *testing.T) Store {
			db, err := newDBStore(*testDSN)
			if err != nil {
				t.Fatalf("cannot create db: %s", err)
			}

			if err := db.ensureSchema(); err != nil {
				t.Fatalf("cannot ensure schema: %s", err)
			}

			if err := db.resetData(); err != nil {
				t.Fatalf("cannot reset data: %s", err)
			}

			return db
		},
	})
}

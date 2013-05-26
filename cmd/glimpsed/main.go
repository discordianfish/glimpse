package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/bmizerany/pat"
)

// Integration type
type Ref struct {
	Job *Job
	Rev int64
}

type Store interface {
	Put(Ref) (Ref, error)
	Get(Path) (*Ref, error)
	Glob(Path) ([]Ref, error)
}

var addr = flag.String("http", ":8411", "address to listen for HTTP requests")

func route(store Store) http.Handler {
	mux := pat.New()
	fields := []string{"zone", "product", "env", "name"}

	mux.Get("/:zone/:product/:env/:name/:index::service", Match(store))
	mux.Get("/:zone/:product/:env/:name::service", Match(store))
	mux.Put("/:zone/:product/:env/:name", JobPut(store))
	mux.Get("/:zone/:product/:env/:name", BrowseOrGet(BrowseServices(store), JobGet(store)))
	mux.Get("/:zone/:product/:env", Browse(store, fields[:3]...))
	mux.Get("/:zone/:product", Browse(store, fields[:2]...))
	mux.Get("/:zone", Browse(store, fields[:1]...))
	return mux
}

func main() {
	log.Fatal(http.ListenAndServe(*addr, route(newMemStore())))
}

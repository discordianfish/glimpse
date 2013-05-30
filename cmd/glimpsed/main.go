package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/bmizerany/pat"
)

var addr = flag.String("http", ":8411", "address to listen for HTTP requests")

func route(store Store) http.Handler {
	mux := pat.New()
	fields := []string{"zone", "product", "env", "name"}

	mux.Get("/:zone/:product/:env/:name/:index::endpoint", MatchOrWatch(Match(store), Watch(store)))
	mux.Get("/:zone/:product/:env/:name::endpoint", MatchOrWatch(Match(store), Watch(store)))
	mux.Put("/:zone/:product/:env/:name", JobPut(store))
	mux.Get("/:zone/:product/:env/:name", BrowseOrGet(BrowseEndpoints(store), JobGet(store)))
	mux.Get("/:zone/:product/:env", Browse(store, fields[:3]...))
	mux.Get("/:zone/:product", Browse(store, fields[:2]...))
	mux.Get("/:zone", Browse(store, fields[:1]...))
	return mux
}

func init() {
	flag.Parse()
}

func main() {
	store, err := newDBStore(*dsn)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(http.ListenAndServe(*addr, guarded(route(store))))
}

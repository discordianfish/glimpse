package main

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
)

func validateJobPath(path Path, job *Job) error {
	if path != job.Path() {
		return fmt.Errorf("expecting job to be at path: %q but should be at %q", path, job.Path())
	}
	return nil
}

func getEtag(r *http.Request) int64 {
	var rev int64
	fmt.Sscan(strings.Trim(r.Header.Get("Etag"), `"`), &rev)
	return rev
}

func setEtag(w http.ResponseWriter, rev int64) {
	w.Header().Set("Etag", fmt.Sprintf(`"%d"`, rev))
}

func JobPut(store Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Content-Type"), "application/x-protobuf") {
			guard(412, fmt.Errorf("expecting application/x-protobuf Content-Type"))
		}

		job, err := DecodeJob(r.Body)
		guard(500, err)

		err = validateJobPath(Path(r.URL.Path), job)
		// FIXME - test validation
		guard(422, err)

		ref, err := store.Put(Ref{job, getEtag(r)})
		// FIXME - test conflict
		guard(409, err)

		// FIXME - test etag round trip
		setEtag(w, ref.Rev)
	})
}

func JobGet(store Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept"), "application/x-protobuf") {
			guard(406, fmt.Errorf("expecting to Accept application/x-protobuf"))
		}

		ref, err := store.Get(Path(r.URL.Path))
		guard(500, err)

		if ref.Job == nil {
			http.Error(w, "", http.StatusNotFound)
			return
		}

		setEtag(w, ref.Rev)
		EncodeJob(w, ref.Job)
	})
}

type glob struct {
	zone     string
	product  string
	env      string
	name     string
	index    string
	endpoint string
}

func (g glob) String() string {
	return "/" + path.Join(g.zone, g.product, g.env, g.name)
}

func (g glob) Path() Path {
	return Path(g.String())
}

func (g glob) ServiceAddress() ServiceAddress {
	return ServiceAddress(g.String() + "/" + g.index + ":" + g.endpoint)
}

func requestToGlob(r *http.Request) glob {
	q := r.URL.Query()
	g := glob{
		zone:    q.Get(":zone"),
		product: q.Get(":product"),
		env:     q.Get(":env"),
		name:    q.Get(":name"),
	}
	if g.zone == "" {
		g.zone = "*"
	}
	if g.product == "" {
		g.product = "*"
	}
	if g.env == "" {
		g.env = "*"
	}
	if g.name == "" {
		g.name = "*"
	}

	g.index = q.Get(":index")
	if g.index == "" {
		g.index = "*"
	}

	g.endpoint = q.Get(":endpoint")
	if len(g.endpoint) > 0 {
		// FIXME stripping colon from this param is a bug in the 'pat' router
		g.endpoint = g.endpoint[1:len(g.endpoint)]
	}
	if g.endpoint == "" {
		g.endpoint = "*"
	}

	return g
}

func MatchOrWatch(match, watch http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "text/event-stream" {
			watch.ServeHTTP(w, r)
		} else {
			match.ServeHTTP(w, r)
		}
	})
}

func flush(w interface{}) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func writeSetEvent(w io.Writer, services []Service) {
	fmt.Fprint(w, "event: set\n")
	for _, srv := range services {
		fmt.Fprintf(w, "data: %s\n", srv.String())
	}
	fmt.Fprint(w, "\n")
	flush(w)
}

func writeChangeEvent(w io.Writer, c Change) error {
	if _, err := fmt.Fprintf(w, "event: %s\n", c.Operation()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", c.Service().String()); err != nil {
		return err
	}

	flush(w)
	return nil
}

func Watch(store Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		glob := requestToGlob(r)
		changes := make(chan Change)

		more := make(chan bool)
		defer close(more)

		services, err := store.Match(glob.ServiceAddress(), func(c Change) bool {
			changes <- c
			return <-more
		})
		guard(500, err)

		writeSetEvent(w, services)
		for c := range changes {
			if err := writeChangeEvent(w, c); err != nil {
				return
			}
			more <- true
		}
	})
}

func Match(store Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		glob := requestToGlob(r)
		services, err := store.Match(glob.ServiceAddress(), nil)
		guard(500, err)

		for _, srv := range services {
			fmt.Fprint(w, srv.String()+"\n")
		}
	})
}

func prefix(path string, count int) string {
	parts := strings.Split(path, "/")
	if len(parts) > count {
		parts = parts[:count]
	}
	return strings.Join(parts, "/")
}

func formatText(w io.Writer, format string, args ...interface{}) error {
	_, err := fmt.Fprintf(w, format, args...)
	return err
}

func formatLink(w io.Writer, format string, args ...interface{}) error {
	item := fmt.Sprintf(format, args...)
	_, err := fmt.Fprintf(w, `<a style="display:block" href="%s">%s</a>`, item, item)
	return err
}

type formatter func(io.Writer, string, ...interface{}) error

func linker(r *http.Request) formatter {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "html") {
		return formatLink
	} else {
		return formatText
	}
}

func BrowseEndpoints(store Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		glob := requestToGlob(r)
		srvs, err := store.Match(glob.ServiceAddress(), nil)
		guard(500, err)

		link := linker(r)
		found := map[string]bool{}
		for _, bind := range srvs {
			name := bind.Endpoint.GetName()
			if !found[name] {
				found[name] = true
				link(w, "%s:%s\n", bind.Job.Path(), name)
			}
		}
	})
}

func Browse(store Store, fields ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		glob := requestToGlob(r)
		srvs, err := store.Match(glob.ServiceAddress(), nil)
		guard(500, err)

		link := linker(r)
		found := map[string]bool{}
		for _, bind := range srvs {
			child := prefix(string(bind.Job.Path()), len(fields)+2)
			if !found[child] {
				found[child] = true
				link(w, "%s\n", child)
			}
		}
	})
}

func BrowseOrGet(browse, get http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "application/x-protobuf") {
			get.ServeHTTP(w, r)
		} else {
			browse.ServeHTTP(w, r)
		}
	})
}

package main

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
)

func catch(w http.ResponseWriter) {
	if err := recover(); err != nil {
		code := http.StatusInternalServerError
		if guard, ok := err.(httpGuard); ok {
			code = guard.code
		}
		http.Error(w, fmt.Sprint(err), code)
	}
}

type httpGuard struct {
	code int
	err  error
}

func (e httpGuard) Error() string { return e.err.Error() }

func guard(code int, err error) {
	if err != nil {
		panic(httpGuard{code, err})
	}
}

func validateJobPath(path Path, job *Job) error {
	if path != job.Path() {
		return fmt.Errorf("expecting job to be at path: %q but should be at %q", path, job.Path())
	}
	return nil
}

func JobPut(store Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer catch(w)

		if !strings.Contains(r.Header.Get("Content-Type"), "application/x-protobuf") {
			guard(412, fmt.Errorf("expecting application/x-protobuf Content-Type"))
		}

		job, err := DecodeJob(r.Body)
		guard(500, err)

		err = validateJobPath(Path(r.URL.Path), job)
		guard(422, err)

		ref, err := store.Put(Ref{job, 0})
		guard(409, err)

		w.Header().Set("Etag", fmt.Sprintf(`"%d"`, ref.Rev))
	})
}

func JobGet(store Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer catch(w)

		if !strings.Contains(r.Header.Get("Accept"), "application/x-protobuf") {
			guard(406, fmt.Errorf("expecting to Accept application/x-protobuf"))
		}

		ref, err := store.Get(Path(r.URL.Path))
		guard(500, err)

		if ref.Job == nil {
			http.Error(w, "", http.StatusNotFound)
			return
		}

		w.Header().Set("Etag", fmt.Sprintf(`"%d"`, ref.Rev))
		EncodeJob(w, ref.Job)
	})
}

type binding struct {
	*Job
	*Instance
	*Service
}

func bindings(refs []Ref) []binding {
	var binds []binding
	for _, ref := range refs {
		if ref.Job != nil {
			for _, inst := range ref.Job.GetInstances() {
				for _, srv := range inst.GetServices() {
					binds = append(binds, binding{ref.Job, inst, srv})
				}
			}
		}
	}
	return binds
}

func matchServices(index, service string, bindings []binding) []binding {
	var binds []binding
	for _, bind := range bindings {
		if index == "" || index == "*" || index == strconv.Itoa(int(bind.Instance.GetIndex())) {
			if service == "*" || service == bind.Service.GetName() {
				binds = append(binds, bind)
			}
		}
	}
	return binds
}

func Match(store Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		glob := "/" + path.Join(q.Get(":zone"), q.Get(":product"), q.Get(":env"), q.Get(":name"))
		index := q.Get(":index")
		service := q.Get(":service")
		if len(service) > 0 { // FIXME stripping colon from service in the wrong place
			service = service[1:len(service)]
		}

		refs, err := store.Glob(Path(glob))
		guard(500, err)

		for _, bind := range matchServices(index, service, bindings(refs)) {
			fmt.Fprintf(w, "%s/%d:%s %s:%d\n",
				bind.Job.Path(),
				bind.Instance.GetIndex(),
				bind.Service.GetName(),
				bind.Service.GetHost(),
				bind.Service.GetPort())
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

func list(w io.Writer, r *http.Request, item string) {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "html") || strings.Contains(accept, "*") {
		fmt.Fprintf(w, `<a style="display:block" href="%s">%s</a>`, item, item)
	} else {
		w.Write([]byte(item))
	}
}

func BrowseServices(store Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ref, err := store.Get(Path(r.URL.Path))
		guard(500, err)

		found := make(map[string]bool)
		for _, bind := range bindings([]Ref{*ref}) {
			name := bind.Service.GetName()
			if !found[name] {
				found[name] = true
				list(w, r, fmt.Sprintf("%s:%s\n", bind.Job.Path(), name))
			}
		}
	})
}

func Browse(store Store, fields ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tmpl := []string{"*", "*", "*", "*"}

		q := r.URL.Query()
		for i, f := range fields {
			tmpl[i] = q.Get(":" + f)
		}
		glob := Path("/" + path.Join(tmpl...))

		refs, err := store.Glob(glob)
		guard(500, err)

		found := make(map[string]bool)
		for _, ref := range refs {
			child := prefix(string(ref.Job.Path()), len(fields)+2)
			if !found[child] {
				found[child] = true
				list(w, r, fmt.Sprintf("%s\n", child))
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

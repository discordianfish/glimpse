package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"code.google.com/p/goprotobuf/proto"
)

func init() {
	tests = append(tests, []acceptanceTest{
		{"Put Job", testPUT},
		{"Get Job", testGET},
		{"Match", testMatch},
		{"Browse", testBrowse},
	}...)
}

func pb(m proto.Message) *bytes.Buffer {
	body, err := proto.Marshal(m)
	if err != nil {
		panic(err)
	}
	return bytes.NewBuffer(body)
}

func testJob(zone, product, env, name string, instances []*Instance) *Job {
	return &Job{
		Zone:     &zone,
		Product:  &product,
		Env:      &env,
		Name:     &name,
		Instance: instances,
	}
}

func testInstance(index uint32, endpoints []*Endpoint) *Instance {
	return &Instance{
		Index:    &index,
		Endpoint: endpoints,
	}
}

func testEndpoint(name string, host string, port uint32) *Endpoint {
	return &Endpoint{
		Name: &name,
		Host: &host,
		Port: &port,
	}
}

func testReq(method, path string, body io.Reader, headers map[string]string) (*http.Request, *httptest.ResponseRecorder) {
	r, err := http.NewRequest(method, path, body)
	if err != nil {
		panic(err)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r, httptest.NewRecorder()
}

func testPUT(t *testing.T, store Store) {
	job := testJob("an", "site", "prod", "api", []*Instance{
		testInstance(0, nil),
	})

	r, w := testReq("PUT", "/an/site/prod/api", pb(job), map[string]string{
		"Content-Type": "application/x-protobuf",
	})

	JobPut(store).ServeHTTP(w, r)

	if 200 != w.Code {
		t.Fatalf("expected PUT to return 200, returned %v %q", w.Code, w.Body.String())
	}

	if ref, err := store.Get(job.Path()); ref == nil || ref.Job.String() != job.String() {
		t.Fatalf("expected to store the job but did not get a match, got: %+v err: %v", ref, err)
	}
}

func testGET(t *testing.T, store Store) {
	job := testJob("an", "site", "prod", "api", []*Instance{
		testInstance(0, nil),
	})
	store.Put(Ref{job, 0})

	r, w := testReq("GET", "/an/site/prod/api", nil, map[string]string{
		"Accept": "application/x-protobuf",
	})

	JobGet(store).ServeHTTP(w, r)

	if 200 != w.Code {
		t.Fatalf("expected PUT to return 200, returned %v %q", w.Code, w.Body.String())
	}

	res, err := DecodeJob(w.Body)
	if err != nil {
		t.Fatalf("expected to decode the test job, got: %v", err)
	}

	if res.String() != job.String() {
		t.Fatalf("expected to get the job but didn't match, got: %q, want: %q", res.String(), job.String())
	}
}

func shouldMatch(t *testing.T, h http.Handler, path string, bodyParts ...string) {
	r, w := testReq("GET", path, nil, nil)
	h.ServeHTTP(w, r)

	if 200 != w.Code {
		t.Fatalf("expected 200, got: %v %q", w.Code, w.Body.String())
	}

	expected := strings.Join(bodyParts, "")
	if w.Body.String() != expected {
		t.Fatalf("expected to match %q, got: %q want: %q", path, w.Body.String(), expected)
	}
}

func testMatch(t *testing.T, store Store) {
	store.Put(Ref{testJob("an", "site", "prod", "api", []*Instance{
		testInstance(0, []*Endpoint{
			testEndpoint("http", "host", 8080),
			testEndpoint("http-mgmt", "host", 8081),
		}),
	}), 0})

	store.Put(Ref{testJob("an", "site", "prod", "worker", []*Instance{
		testInstance(0, []*Endpoint{
			testEndpoint("http", "host", 8080),
		}),
	}), 0})

	h := route(store)

	shouldMatch(t, h, "/an/site/prod/api/0:http",
		"/an/site/prod/api/0:http host:8080\n")

	shouldMatch(t, h, "/an/site/prod/api/0:http-mgmt",
		"/an/site/prod/api/0:http-mgmt host:8081\n")

	shouldMatch(t, h, "/an/site/prod/api/0:*",
		"/an/site/prod/api/0:http host:8080\n",
		"/an/site/prod/api/0:http-mgmt host:8081\n")

	shouldMatch(t, h, "/an/site/prod/*/*:http",
		"/an/site/prod/api/0:http host:8080\n",
		"/an/site/prod/worker/0:http host:8080\n")

	// equivalent to above
	shouldMatch(t, h, "/an/site/prod/*:http",
		"/an/site/prod/api/0:http host:8080\n",
		"/an/site/prod/worker/0:http host:8080\n")

	shouldMatch(t, h, "/an/site/prod/*/*:*",
		"/an/site/prod/api/0:http host:8080\n",
		"/an/site/prod/api/0:http-mgmt host:8081\n",
		"/an/site/prod/worker/0:http host:8080\n")
}

func testBrowse(t *testing.T, store Store) {
	store.Put(Ref{testJob("an", "site", "prod", "api", []*Instance{
		testInstance(0, []*Endpoint{
			testEndpoint("http", "host", 8080),
			testEndpoint("http-mgmt", "host", 8081),
		}),
	}), 0})

	store.Put(Ref{testJob("an", "site", "prod", "worker", []*Instance{
		testInstance(0, []*Endpoint{
			testEndpoint("http", "host", 8080),
		}),
	}), 0})

	store.Put(Ref{testJob("an", "extra", "prod", "api", []*Instance{
		testInstance(0, []*Endpoint{
			testEndpoint("http", "host", 8080),
		}),
	}), 0})

	h := route(store)

	shouldMatch(t, h, "/an",
		"/an/extra\n",
		"/an/site\n")

	shouldMatch(t, h, "/an/site",
		"/an/site/prod\n")

	shouldMatch(t, h, "/an/site/prod",
		"/an/site/prod/api\n",
		"/an/site/prod/worker\n")

	shouldMatch(t, h, "/an/site/prod/api",
		"/an/site/prod/api:http\n",
		"/an/site/prod/api:http-mgmt\n")
}

func shouldStreamMessage(t *testing.T, buf *bytes.Buffer, lines ...string) {
	r := bytes.NewBuffer(buf.Bytes())
	for _, line := range append(lines, "\n") {
		got, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("expected to stream message, got err %v", err)
		}
		if got != line {
			t.Fatalf("expected to stream line: %q, got %q", line, got)
		}
	}
	buf.Reset()
}

func testWatch(t *testing.T, store Store) {
	store.Put(Ref{testJob("an", "site", "prod", "api", []*Instance{
		testInstance(0, []*Endpoint{
			testEndpoint("http", "host", 8080),
			testEndpoint("http-mgmt", "host", 8081),
		}),
	}), 0})

	store.Put(Ref{testJob("an", "site", "prod", "worker", []*Instance{
		testInstance(0, []*Endpoint{
			testEndpoint("http", "host", 8080),
		}),
	}), 0})

	store.Put(Ref{testJob("an", "site", "prod", "api", []*Instance{
		testInstance(0, []*Endpoint{
			testEndpoint("http", "host", 8080),
		}),
	}), 0})

	r, w := testReq("GET", "/an/site/prod/*/*:*", nil, map[string]string{
		"Accept": "text/event-stream",
	})

	go route(store).ServeHTTP(w, r)

	// FIXME synchronize the long poll
	time.Sleep(time.Millisecond)

	if 200 != w.Code {
		t.Fatalf("expected 200, got: %v %q", w.Code, w.Body.String())
	}

	shouldStreamMessage(t, w.Body,
		"event: set\n",
		"data: /an/site/prod/api/0:http host:8080\n",
		"data: /an/site/prod/worker/0:http host:8080\n",
	)

	store.Put(Ref{testJob("an", "site", "prod", "lolz", []*Instance{
		testInstance(0, []*Endpoint{
			testEndpoint("http", "host", 8080),
		}),
	}), 0})

	// FIXME synchronize the long poll
	time.Sleep(time.Millisecond)

	shouldStreamMessage(t, w.Body,
		"event: add\n",
		"data: /an/site/prod/lolz/0:http host:8080\n",
	)

	// FIXME test deletion messages
}

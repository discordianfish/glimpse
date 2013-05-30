package main

import (
	"fmt"
	"net/http"
)

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

func guarded(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				code := http.StatusInternalServerError
				if guard, ok := err.(httpGuard); ok {
					code = guard.code
				}
				http.Error(w, fmt.Sprint(err), code)
			}
		}()

		h.ServeHTTP(w, r)
	})
}

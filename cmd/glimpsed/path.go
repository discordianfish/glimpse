package main

import (
	"strings"
)

type Path string

func (p Path) String() string { return string(p) }

func (p Path) Parts() (zone string, product string, env string, name string) {
	parts := strings.Split(string(p), "/")
	if len(parts) > 1 {
		zone = parts[1]
	}
	if len(parts) > 2 {
		product = parts[2]
	}
	if len(parts) > 3 {
		env = parts[3]
	}
	if len(parts) > 4 {
		name = parts[4]
	}
	return
}

package main

import "net"

type instance struct {
	info info
	host string
	ip   net.IP
	port uint16
}

type instances []*instance

type store interface {
	getInstances(info) (instances, error)
}

type info struct {
	env      string
	job      string
	product  string
	provider string
	service  string
	zone     string
}

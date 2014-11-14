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

// TODO(alx): Find better naming.
// TODO(alx): evaluate if provider has a place here.
// TODO(alx): Potentially hardening with concrete types having Validate methods
//						instead of strings.
// Code struct for service address: "job.task.env.product".
type info struct {
	env      string
	job      string
	product  string
	provider string
	service  string
	zone     string
}

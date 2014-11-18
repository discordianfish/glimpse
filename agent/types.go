package main

import (
	"fmt"
	"net"
	"strings"
)

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

func infoFromAddr(addr string) (info, error) {
	fields := strings.SplitN(addr, ".", 5)

	if len(fields) != 5 {
		return info{}, fmt.Errorf("invalid service address: %s", addr)
	}

	var (
		zone    = fields[4]
		product = fields[3]
		env     = fields[2]
		job     = fields[1]
		service = fields[0]
	)

	if len(zone) > 1 && !rZone.MatchString(zone) {
		return info{}, fmt.Errorf("zone %q is invalid", zone)
	}
	if !rField.MatchString(product) {
		return info{}, fmt.Errorf("product %q is invalid", product)
	}
	if !rField.MatchString(env) {
		return info{}, fmt.Errorf("env %q is invalid", env)
	}
	if !rField.MatchString(job) {
		return info{}, fmt.Errorf("job %q is invalid", job)
	}
	if !rField.MatchString(service) {
		return info{}, fmt.Errorf("service %q is invalid", service)
	}

	return info{
		env:     env,
		job:     job,
		product: product,
		service: service,
		zone:    zone,
	}, nil
}

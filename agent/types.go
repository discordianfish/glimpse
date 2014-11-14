package main

import "net"

type instance struct {
	srvInfo srvInfo
	host    string
	ip      net.IP
	port    uint16
}

type instances []*instance

type store interface {
	getInstances(s srvInfo) (instances, error)
}

type srvInfo struct {
	env     string
	job     string
	product string
	service string
	zone    string
}

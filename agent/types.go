package main

import "net"

type instance struct {
	srvInfo srvInfo
	host    string
	ip      net.IP
	port    uint16
}

type store interface {
	getInstances(s srvInfo) ([]*instance, error)
}

type srvInfo struct {
	env     string
	job     string
	product string
	service string
	zone    string
}

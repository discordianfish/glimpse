package main

import (
	"fmt"
	"log"
	"net"

	"github.com/armon/consul-api"
)

type consulStore struct {
	client *consulapi.Client
}

func (s *consulStore) getInstances(srv srvInfo) ([]*instance, error) {
	var (
		envTag     = fmt.Sprintf("glimpse:env=%s", srv.env)
		jobTag     = fmt.Sprintf("glimpse:job=%s", srv.job)
		serviceTag = fmt.Sprintf("glimpse:service=%s", srv.service)
		options    = &consulapi.QueryOptions{
			AllowStale: true,
			Datacenter: srv.zone,
		}

		nodes = []*instance{}
	)

	catalog := s.client.Catalog()
	allNodes, meta, err := catalog.Service(srv.product, jobTag, options)
	if err != nil {
		return nil, err
	}

	log.Printf(
		"consul lookup of %s.*.%s took %dns\n",
		srv.product,
		srv.job,
		meta.RequestTime.Nanoseconds(),
	)

	for _, node := range allNodes {
		var (
			isEnv     bool
			isService bool
		)

		for _, tag := range node.ServiceTags {
			if tag == envTag {
				isEnv = true
			}
			if tag == serviceTag {
				isService = true
			}
		}

		if isEnv && isService {
			ins := &instance{
				srvInfo: srv,
				host:    node.Address,
				ip:      net.ParseIP(node.Node),
				port:    uint16(node.ServicePort),
			}
			nodes = append(nodes, ins)
		}
	}

	return nodes, nil
}

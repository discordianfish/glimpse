package main

import (
	"fmt"
	"net"

	"github.com/armon/consul-api"
)

type consulStore struct {
	client *consulapi.Client
}

func (s *consulStore) getInstances(srv srvInfo) (instances, error) {
	var (
		envTag     = fmt.Sprintf("glimpse:env=%s", srv.env)
		jobTag     = fmt.Sprintf("glimpse:job=%s", srv.job)
		serviceTag = fmt.Sprintf("glimpse:service=%s", srv.service)
		options    = &consulapi.QueryOptions{
			AllowStale: true,
			Datacenter: srv.zone,
		}

		is = []*instance{}
	)

	nodes, _, err := s.client.Catalog().Service(srv.product, jobTag, options)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
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
			is = append(is, &instance{
				srvInfo: srv,
				host:    node.Address,
				ip:      net.ParseIP(node.Node),
				port:    uint16(node.ServicePort),
			})
		}
	}

	return is, nil
}

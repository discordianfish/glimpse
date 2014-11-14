package main

import (
	"fmt"
	"net"

	"github.com/armon/consul-api"
)

type consulStore struct {
	client *consulapi.Client
}

func newConsulStore(client *consulapi.Client) store {
	return &consulStore{
		client: client,
	}
}

func (s *consulStore) getInstances(info srvInfo) (instances, error) {
	var (
		envTag     = fmt.Sprintf("glimpse:env=%s", info.env)
		jobTag     = fmt.Sprintf("glimpse:job=%s", info.job)
		serviceTag = fmt.Sprintf("glimpse:service=%s", info.service)
		options    = &consulapi.QueryOptions{
			AllowStale: true,
			Datacenter: info.zone,
		}

		is = []*instance{}
	)

	nodes, _, err := s.client.Catalog().Service(info.product, jobTag, options)
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
				info: info,
				host: node.Address,
				ip:   net.ParseIP(node.Node),
				port: uint16(node.ServicePort),
			})
		}
	}

	return is, nil
}

func infoToTags(info srvInfo) []string {
	return []string{
		fmt.Sprintf("glimpse:env=%s", info.env),
		fmt.Sprintf("glimpse:job=%s", info.job),
		fmt.Sprintf("glimpse:product=%s", info.product),
		fmt.Sprintf("glimpse:provider=%s", info.provider),
		fmt.Sprintf("glimpse:service=%s", info.service),
	}
}

package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/consul/api"
	consul "github.com/hashicorp/consul/consul/structs"
)

type consulStore struct {
	client *api.Client
}

func newConsulStore(client *api.Client) store {
	return &consulStore{
		client: client,
	}
}

func (s *consulStore) getInstances(info info) (instances, error) {
	var (
		envTag      = fmt.Sprintf("glimpse:env=%s", info.env)
		jobTag      = fmt.Sprintf("glimpse:job=%s", info.job)
		serviceTag  = fmt.Sprintf("glimpse:service=%s", info.service)
		passingOnly = true
		options     = &api.QueryOptions{
			AllowStale: true,
			Datacenter: info.zone,
		}

		is = instances{}
	)

	// As the default we only retrieve healthy instances. Returning different
	// sub/super-sets should be done through different methods communicated clearly
	// the expected response.
	entries, _, err := s.client.Health().Service(info.product, jobTag, passingOnly, options)
	if err != nil {
		return nil, newError(errConsulAPI, "%s", err)
	}

	entries = filterEntries(entries)

	if len(entries) == 0 {
		return nil, newError(errNoInstances, "found for %s", info.addr())
	}

	for _, e := range entries {
		var (
			isEnv     bool
			isService bool
		)

		for _, tag := range e.Service.Tags {
			if tag == envTag {
				isEnv = true
			}
			if tag == serviceTag {
				isService = true
			}
		}

		ip := net.ParseIP(e.Node.Address)
		if ip == nil {
			return nil, newError(errInvalidIP, "parse failed for %s", e.Node.Address)
		}

		if isEnv && isService {
			is = append(is, instance{
				host: e.Node.Node,
				ip:   ip,
				port: uint16(e.Service.Port),
			})
		}
	}

	return is, nil
}

func (s *consulStore) getServers(zone string) (instances, error) {
	members, err := s.client.Agent().Members(true)
	if err != nil {
		return nil, newError(errConsulAPI, "%s", err)
	}
	srvs := instances{}
	for _, m := range members {
		if zone == "" || strings.HasSuffix(m.Name, "."+zone) {
			n := m.Name
			if i := strings.LastIndex(n, "."); i > 0 {
				n = n[:i]
			}
			srvs = append(srvs, instance{ip: net.ParseIP(m.Addr), host: n})
		}
	}
	return srvs, nil
}

func filterEntries(entries []*api.ServiceEntry) []*api.ServiceEntry {
	if len(entries) == 0 {
		return entries
	}

	es := []*api.ServiceEntry{}

	for _, e := range entries {
		isHealthy := true

		for _, check := range e.Checks {
			if check.Status == consul.HealthCritical {
				isHealthy = false
			}
		}

		if isHealthy {
			es = append(es, e)
		}
	}

	return es
}

func infoToTags(info info) []string {
	return []string{
		fmt.Sprintf("glimpse:env=%s", info.env),
		fmt.Sprintf("glimpse:job=%s", info.job),
		fmt.Sprintf("glimpse:product=%s", info.product),
		fmt.Sprintf("glimpse:provider=%s", info.provider),
		fmt.Sprintf("glimpse:service=%s", info.service),
	}
}

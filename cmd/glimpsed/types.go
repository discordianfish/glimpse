package main

import (
	"fmt"
	"path"
)

type Op string

const (
	Add Op = "add"
	Del Op = "del"
)

type Change interface {
	Operation() Op
	Service() *Service
}

type ServiceAddress string

func (p ServiceAddress) Match(o ServiceAddress) bool {
	ok, _ := path.Match(string(p), string(o))
	return ok
}

func (p ServiceAddress) JobPath() string {
	return path.Dir(string(p))
}

type Service struct {
	*Job
	*Instance
	*Endpoint
}

func (s Service) Address() ServiceAddress {
	return ServiceAddress(fmt.Sprintf("/%s/%s/%s/%s/%d:%s",
		s.Job.GetZone(),
		s.Job.GetProduct(),
		s.Job.GetEnv(),
		s.Job.GetName(),
		s.Instance.GetIndex(),
		s.Endpoint.GetName(),
	))
}

func (s Service) String() string {
	return fmt.Sprintf("%s %s:%d",
		s.Address(),
		s.Endpoint.GetHost(),
		s.Endpoint.GetPort(),
	)
}

type ServiceGroup []Service

func (g ServiceGroup) Less(i, j int) bool { return g[i].String() < g[j].String() }
func (g ServiceGroup) Swap(i, j int)      { g[i], g[j] = g[j], g[i] }
func (g ServiceGroup) Len() int           { return len(g) }

type Ref struct {
	*Job
	Rev int64
}

type WatchFunc func(Change) bool

type Store interface {
	Put(Ref) (Ref, error)
	Get(Path) (*Ref, error)
	Match(ServiceAddress, WatchFunc) ([]Service, error)
}

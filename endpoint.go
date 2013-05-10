// Copyright (c) 2013, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/glimpse

package glimpse

import (
	"fmt"
	cp "github.com/soundcloud/cotterpin"
	"net"
)

type Endpoint struct {
	file       *cp.File `json:"-"`
	Service    *Service `json:"-"`
	IP         string   `json:"ip"`
	Port       uint16   `json:"port"`
	Priority   uint16   `json:"priority,omitempty"`
	Weight     uint16   `json:"weight,omitempty"`
	Hostname   string   `json:"target,omitempty"`
	Labels     []string `json:"labels"`
	Registered string   `json:"registered"`
}

func (s *Service) NewEndpoint(ip string, port uint16, hostname string) *Endpoint {
	e := &Endpoint{
		Service:  s,
		IP:       ip,
		Port:     port,
		Hostname: hostname,
	}
	e.file = cp.NewFile(e.path(), e, new(cp.JsonCodec), s.GetSnapshot())

	return e
}

func (e *Endpoint) GetSnapshot() cp.Snapshot {
	return e.file.Snapshot
}

// Register adds the Service to the global directory.
func (e *Endpoint) Register() (*Endpoint, error) {
	ip := net.ParseIP(e.IP)
	if ip == nil {
		return nil, ErrInvalidIP
	}

	sp, err := e.GetSnapshot().FastForward()
	if err != nil {
		return nil, err
	}

	exists, _, err := sp.Exists(e.path())
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrConflict
	}

	e.Registered = timestamp()

	f := cp.NewFile(e.path(), e, new(cp.JsonCodec), sp)
	f, err = f.Save()
	if err != nil {
		return nil, err
	}
	e.file = f

	return e, nil
}

// Unregister removes the Endpoint from the global directory.
func (e *Endpoint) Unregister() error {
	return e.file.Del()
}

// WaitUnregister blocks until the Endpoint is unregistered
func (e *Endpoint) WaitUnregister() error {
	sp := e.GetSnapshot()
	for {
		ev, err := sp.Wait(e.path())
		if err != nil {
			return err
		}
		if ev.IsDel() {
			return nil
		} else if ev.IsSet() {
			sp = sp.Join(ev)
		}
	}

	return nil
}

func (e *Endpoint) String() string {
	f := "Endpoint<%s>{IP: %s, Port: %d, Priority: %d, Weight: %d, Hostname: %s, Registered: %s}"
	return fmt.Sprintf(f, e.id(), e.IP, e.Port, e.Priority, e.Weight, e.Hostname, e.Registered)
}

func (e *Endpoint) path() string {
	return e.Service.endpointPath(e.id())
}

func (e *Endpoint) id() string {
	return fmt.Sprintf("%s-%d", e.IP, e.Port)
}

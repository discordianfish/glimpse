// Copyright (c) 2013, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/glimpse

package glimpse

import (
	"fmt"
	cp "github.com/soundcloud/cotterpin"
	"path"
	"strings"
)

const (
	endpointsPath  string = "endpoints"
	registeredPath string = "registered"
	servicesPath   string = "/services"
)

type Service struct {
	dir  *cp.Dir
	Name string
}

func (d *Directory) NewService(name string) *Service {
	s := &Service{Name: name}
	s.dir = cp.NewDir(path.Join(servicesPath, name), d.GetSnapshot())
	return s
}

func (s *Service) GetSnapshot() cp.Snapshot {
	return s.dir.GetSnapshot()
}

// Register adds the Service to the global directory.
func (s *Service) Register() (*Service, error) {
	sp, err := s.dir.GetSnapshot().FastForward()
	if err != nil {
		return nil, err
	}

	exists, _, err := sp.Exists(s.dir.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrConflict
	}

	d, err := s.dir.Join(sp).Set(registeredPath, timestamp())
	if err != nil {
		return nil, err
	}
	s.dir = d

	return s, nil
}

// Unregister removes the Service from the global directory.
func (s *Service) Unregister() error {
	sp, err := s.GetSnapshot().FastForward()
	if err != nil {
		return err
	}
	return s.dir.Join(sp).Del("/")
}

// GetEndpoint fetches the Endpoint for the given id.
func (s *Service) GetEndpoint(id string) (*Endpoint, error) {
	codec := new(cp.JsonCodec)
	codec.DecodedVal = &Endpoint{}
	sp, err := s.GetSnapshot().FastForward()
	if err != nil {
		return nil, err
	}
	f, err := sp.GetFile(s.endpointPath(id), codec)
	if err != nil {
		return nil, err
	}
	e := f.Value.(*Endpoint)
	e.file = f
	e.Service = s
	return e, nil
}

// GetEndpoints fetches all Endpoints of the Service
func (s *Service) GetEndpoints() ([]*Endpoint, error) {
	sp, err := s.GetSnapshot().FastForward()
	if err != nil {
		return nil, err
	}
	ids, err := sp.Getdir(s.dir.Prefix(endpointsPath))
	if err != nil {
		return nil, err
	}
	ch, errch := cp.GetSnapshotables(ids, func(id string) (cp.Snapshotable, error) {
		return s.GetEndpoint(id)
	})

	eps := []*Endpoint{}
	for _ = range ids {
		select {
		case e := <-ch:
			eps = append(eps, e.(*Endpoint))
		case err := <-errch:
			return nil, err
		}
	}
	return eps, nil
}

// WaitUnregister blocks until the Service is unregistered.
func (s *Service) WaitUnregister() error {
	sp := s.GetSnapshot()
	for {
		ev, err := sp.Wait(s.dir.Prefix(registeredPath))
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

// WaitEndpointRegister blocks until a new Endpoint is registered
// for the Service and returns the new Endpoint.
func (s *Service) WaitEndpointRegister() (*Endpoint, error) {
	sp := s.GetSnapshot()
	for {
		ev, err := sp.Wait(path.Join(s.dir.Prefix(endpointsPath), "*"))
		if err != nil {
			return nil, err
		}
		sp = sp.Join(ev)
		if ev.IsSet() {
			id := strings.SplitN(ev.Path, "/", 5)[4]
			return s.GetEndpoint(id)
		}
	}
	return &Endpoint{}, nil
}

// WaitEndpointRegister blocks until an Endpoint is unregistered
// for the Service and returns the id.
func (s *Service) WaitEndpointUnregister() (string, error) {
	sp := s.GetSnapshot()
	for {
		ev, err := sp.Wait(path.Join(s.dir.Prefix(endpointsPath), "*"))
		if err != nil {
			return "", err
		}
		sp = sp.Join(ev)
		if ev.IsDel() {
			id := strings.SplitN(ev.Path, "/", 5)[4]
			return id, nil
		}
	}

	return "", nil
}

func (s *Service) String() string {
	return fmt.Sprintf("Service<%s>{}", s.Name)
}

func (s *Service) getDir() *cp.Dir {
	return s.dir
}

func (s *Service) endpointPath(id string) string {
	return s.dir.Prefix(path.Join(endpointsPath, id))
}

// GetService fetches the Service for the given name.
func (d *Directory) GetService(name string) (*Service, error) {
	d, err := d.fastForward()
	if err != nil {
		return nil, err
	}

	s := d.NewService(name)
	check, _, err := d.GetSnapshot().Exists(s.dir.Prefix(registeredPath))
	if err != nil {
		return nil, err
	}
	if !check {
		return nil, ErrNotFound
	}
	return s, nil
}

// Services returns the list of all registered Services.
func (d *Directory) GetServices() ([]*Service, error) {
	d, err := d.fastForward()
	if err != nil {
		return nil, err
	}
	check, _, err := d.GetSnapshot().Exists(servicesPath)
	if err != nil {
		return nil, err
	}
	if !check {
		return []*Service{}, nil
	}

	names, err := d.GetSnapshot().Getdir(servicesPath)
	if err != nil {
		return nil, err
	}

	ch, errch := cp.GetSnapshotables(names, func(name string) (cp.Snapshotable, error) {
		return d.GetService(name)
	})

	srvs := []*Service{}
	for _ = range names {
		select {
		case s := <-ch:
			srvs = append(srvs, s.(*Service))
		case err := <-errch:
			return nil, err
		}
	}
	return srvs, nil
}

// WaitServiceRegister blocks until a new Service is registered
// and returns the new Service.
func (d *Directory) WaitServiceRegister() (*Service, error) {
	sp := d.GetSnapshot()
	for {
		ev, err := sp.Wait(path.Join(servicesPath, "*", registeredPath))
		if err != nil {
			return nil, err
		}
		sp = sp.Join(ev)
		if ev.IsSet() {
			name := strings.SplitN(ev.Path, "/", 4)[2]
			return d.GetService(name)
		}
	}
	return &Service{}, nil
}

// Copyright (c) 2013, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/glimpse

package glimpse

import (
	"testing"
)

func setupService(name string, t *testing.T) (*Directory, *Service) {
	d, err := DialUri(DefaultUri, "/service-test")
	if err != nil {
		t.Fatal(err)
	}
	err = d.reset()
	if err != nil {
		t.Fatal(err)
	}
	d, err = d.FastForward()
	if err != nil {
		t.Fatal(err)
	}
	s := d.NewService(name)

	return d, s
}

func TestServiceRegistration(t *testing.T) {
	_, s := setupService("srv-register", t)

	check, _, err := s.dir.GetSnapshot().Exists(s.dir.Name)
	if err != nil {
		t.Fatal(err)
	}
	if check {
		t.Fatal("Service already exists")
	}

	s2, err := s.Register()
	if err != nil {
		t.Fatal(err)
	}
	check, _, err = s2.GetSnapshot().Exists(s.dir.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !check {
		t.Fatal("Service registration failed")
	}

	_, err = s.Register()
	if err == nil {
		t.Error("Service registered twice")
	}
	_, err = s2.Register()
	if err == nil {
		t.Error("Service registered twice")
	}
}

func TestServiceUnregistration(t *testing.T) {
	d, s := setupService("srv-unregister", t)

	s, err := s.Register()
	if err != nil {
		t.Fatal(err)
	}
	err = s.Unregister()
	if err != nil {
		t.Fatal(err)
	}

	d, err = d.FastForward()
	if err != nil {
		t.Fatal(err)
	}
	s = s.Join(d)

	check, _, err := s.GetSnapshot().Exists(s.dir.Name)
	if err != nil {
		t.Fatal(err)
	}
	if check {
		t.Error("Service still registered")
	}
}

func TestServiceUnregistrationFailure(t *testing.T) {
	d, s := setupService("srv-unregister-fail", t)

	s2, err := s.Register()
	if err != nil {
		t.Fatal(err)
	}
	err = s.Unregister()
	if err == nil {
		t.Fatal("Service allowed to unregister with old revision")
	}
	err = s2.Unregister()
	if err != nil {
		t.Fatal(err)
	}

	d, err = d.FastForward()
	if err != nil {
		t.Fatal(err)
	}
	s3 := s2.Join(d)
	_, err = s3.Register()
	if err != nil {
		t.Fatal(err)
	}
}

func TestServiceGetEndpoints(t *testing.T) {
	d, s := setupService("srv-getendpoints", t)
	ids := map[string]bool{"1.2.3.4-8000": true, "1.2.3.4-8001": true, "1.2.3.4-8002": true}

	s, err := s.Register()
	if err != nil {
		t.Fatal(err)
	}
	d = d.Join(s)
	for i := 0; i < len(ids); i++ {
		e := d.NewEndpoint(s, "1.2.3.4", uint16(8000+i), "oddhost.com")
		e, err := e.Register()
		if err != nil {
			t.Fatal(err)
		}
		s = s.Join(e)
	}

	eps, err := s.GetEndpoints()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != len(eps) {
		t.Fatalf("expected length %d returned length %d", len(ids), len(eps))
	}
	for _, ep := range eps {
		if !ids[ep.id()] {
			t.Errorf("expected %s to be in %s", ep.id(), ids)
		}
	}
}

func TestServiceWaitUnregister(t *testing.T) {
	_, s := setupService("srv-wait-unregister", t)
	s1, err := s.Register()
	if err != nil {
		t.Fatal(err)
	}
	err = s1.Unregister()
	if err != nil {
		t.Fatal(err)
	}
	err = s.WaitUnregister()
	if err != nil {
		t.Fatal(err)
	}
}

func TestServiceWaitEndpointRegister(t *testing.T) {
	d, s := setupService("srv-wait-endpoint-register", t)
	s, err := s.Register()
	if err != nil {
		t.Fatal(err)
	}
	d = d.Join(s)
	ep, err := d.NewEndpoint(s, "1.2.3.4", 1234, "example.com").Register()
	if err != nil {
		t.Fatal(err)
	}
	ep1, err := s.WaitEndpointRegister()
	if err != nil {
		t.Fatal(err)
	}
	if ep.String() != ep1.String() {
		t.Errorf("expected %s got %s", ep, ep1)
	}
}

func TestServiceWaitEndpointUnregister(t *testing.T) {
	d, s := setupService("srv-wait-endpoint-unregister", t)
	s, err := s.Register()
	if err != nil {
		t.Fatal(err)
	}
	d = d.Join(s)
	ep, err := d.NewEndpoint(s, "1.2.3.4", 1234, "example.com").Register()
	if err != nil {
		t.Fatal(err)
	}
	err = ep.Unregister()
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.WaitEndpointUnregister()
	if err != nil {
		t.Fatal(err)
	}
	if ep.id() != id {
		t.Errorf("expected %s got %s", ep.id(), id)
	}
}

func TestWaitServiceRegister(t *testing.T) {
	d, s := setupService("wait-srv-register", t)

	s, err := s.Register()
	if err != nil {
		t.Fatal(err)
	}
	s1, err := d.WaitServiceRegister()
	if err != nil {
		t.Fatal(err)
	}
	if s.String() != s1.String() {
		t.Errorf("expected %s got %s", s, s1)
	}
}

func TestServices(t *testing.T) {
	d, _ := setupService("srv-list", t)
	names := map[string]bool{"one": true, "two": true, "three": true}

	for name := range names {
		s := d.NewService(name)
		s, err := s.Register()
		if err != nil {
			t.Fatal(err)
		}
		d = d.Join(s)
	}

	srvs, err := d.Services()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != len(srvs) {
		t.Fatalf("expected length %d returned length %d", len(names), len(srvs))
	}
	for _, srv := range srvs {
		if !names[srv.Name] {
			t.Errorf("expected %s to be in %s", srv.Name, names)
		}
	}
}

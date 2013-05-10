// Copyright (c) 2013, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/glimpse

package glimpse

import (
	"testing"
)

func setupEndpoint(ip string, port uint16, target string, sName string, t *testing.T) (*Directory, *Endpoint) {
	d, err := DialUri(DefaultUri, "/endpoint-test")
	if err != nil {
		t.Fatal(err)
	}
	err = d.reset()
	if err != nil {
		t.Fatal(err)
	}
	d, err = d.fastForward()
	if err != nil {
		t.Fatal(err)
	}
	s := d.NewService(sName)

	e := d.NewEndpoint(s, ip, port, target)

	return d, e
}

func TestEndpointRegistration(t *testing.T) {
	_, e := setupEndpoint("1.2.3.4", 1234, "host.com", "endpoint-register", t)

	eFake := *e
	eFake.IP = "wrong-ip"
	_, err := eFake.Register()
	if err == nil || !IsErrInvalidIP(err) {
		t.Error("Endpoint allowed to register with wrong ip")
	}

	e2, err := e.Register()
	if err != nil {
		t.Fatal(err)
	}
	check, _, err := e2.GetSnapshot().Exists(e.path())
	if err != nil {
		t.Fatal(err)
	}
	if !check {
		t.Fatal("Endpoint registration failed")
	}

	_, err = e.Register()
	if err == nil {
		t.Error("Endpoint registered twice")
	}
	_, err = e2.Register()
	if err == nil {
		t.Error("Endpoint registered twice")
	}
}

func TestEndpointUnregistration(t *testing.T) {
	_, e := setupEndpoint("4.3.2.1", 4321, "hosted.com", "endpoint-unregister", t)

	e2, err := e.Register()
	if err != nil {
		t.Fatal(err)
	}
	err = e2.Unregister()
	if err != nil {
		t.Fatal(err)
	}

	sp, err := e.GetSnapshot().FastForward()
	if err != nil {
		t.Fatal(err)
	}

	check, _, err := sp.Exists(e.path())
	if err != nil {
		t.Fatal(err)
	}
	if check {
		t.Error("Endpoint still registered")
	}
}

func TestEndpointWaitUnregister(t *testing.T) {
	_, e := setupEndpoint("2.3.4.5", 2134, "unhosted.com", "endpoint-wait-unregister", t)

	e1, err := e.Register()
	if err != nil {
		t.Fatal(err)
	}
	err = e1.Unregister()
	if err != nil {
		t.Fatal(err)
	}
	err = e.WaitUnregister()
	if err != nil {
		t.Fatal(err)
	}
}

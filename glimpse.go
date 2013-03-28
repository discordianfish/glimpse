// Copyright (c) 2013, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/glimpse

package glimpse

import (
	cp "github.com/soundcloud/cotterpin"
	"time"
)

const SchemaVersion = 1

const (
	DefaultUri  string = "doozer:?ca=localhost:8046"
	DefaultRoot string = "/glimpse"
)

type Directory struct {
	snapshot cp.Snapshot
}

func DialUri(uri, root string) (*Directory, error) {
	snapshot, err := cp.DialUri(uri, root)
	if err != nil {
		return nil, err
	}
	return &Directory{snapshot}, nil
}

func (d *Directory) GetSnapshot() cp.Snapshot {
	return d.snapshot
}

// Join advances the Directory in time. It returns a new
// instance of Directory at the rev of the supplied
// cp.Snapshotable.
func (d *Directory) Join(sp cp.Snapshotable) *Directory {
	tmp := *d
	tmp.snapshot = sp.GetSnapshot()
	return &tmp
}

func (d *Directory) FastForward() (*Directory, error) {
	s, err := d.GetSnapshot().FastForward()
	if err != nil {
		return nil, err
	}
	return &Directory{s}, nil
}

// Init takes care of basic setup of the tree in the
// coordinator.
func (d *Directory) Init() (*Directory, error) {
	sp, err := cp.SetSchemaVersion(SchemaVersion, d.GetSnapshot())
	if err != nil {
		return nil, err
	}
	d = d.Join(sp)

	return d, nil
}

func (d *Directory) reset() error {
	return d.GetSnapshot().Reset()
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

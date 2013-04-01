// Copyright (c) 2013, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/glimpse

package glimpse

import (
	"errors"
)

var (
	ErrConflict  = errors.New("object already exists")
	ErrInvalidIP = errors.New("provided ip is not valid")
	ErrNotFound  = errors.New("object not found")
)

func IsErrConflict(e error) bool {
	return e == ErrConflict
}

func IsErrInvalidIP(e error) bool {
	return e == ErrInvalidIP
}

func IsErrNotFound(e error) bool {
	return e == ErrNotFound
}

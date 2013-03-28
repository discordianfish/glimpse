package glimpse

import (
	"errors"
)

var (
	ErrConflict  = errors.New("object already exsits")
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

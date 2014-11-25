package main

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

var (
	errConsulAPI   = errors.New("consulAPI")
	errInvalidIP   = errors.New("invalidIP")
	errNoInstances = errors.New("noInstances")
	errUntracked   = errors.New("untracked")
)

type store interface {
	getInstances(info) (instances, error)
}

type instances []*instance

type instance struct {
	info info
	host string
	ip   net.IP
	port uint16
}

// TODO(alx): Find better naming.
// TODO(alx): evaluate if provider has a place here.
// TODO(alx): Potentially hardening with concrete types having Validate methods
//						instead of strings.
// Code struct for service address: "job.task.env.product".
type info struct {
	env      string
	job      string
	product  string
	provider string
	service  string
	zone     string
}

func infoFromAddr(addr string) (info, error) {
	fields := strings.SplitN(addr, ".", 5)

	if len(fields) != 5 {
		return info{}, fmt.Errorf("invalid service address: %s", addr)
	}

	var (
		zone    = fields[4]
		product = fields[3]
		env     = fields[2]
		job     = fields[1]
		service = fields[0]
	)

	if len(zone) > 1 && !rZone.MatchString(zone) {
		return info{}, fmt.Errorf("zone %q is invalid", zone)
	}
	if !rField.MatchString(product) {
		return info{}, fmt.Errorf("product %q is invalid", product)
	}
	if !rField.MatchString(env) {
		return info{}, fmt.Errorf("env %q is invalid", env)
	}
	if !rField.MatchString(job) {
		return info{}, fmt.Errorf("job %q is invalid", job)
	}
	if !rField.MatchString(service) {
		return info{}, fmt.Errorf("service %q is invalid", service)
	}

	return info{
		env:     env,
		job:     job,
		product: product,
		service: service,
		zone:    zone,
	}, nil
}

func (i info) addr() string {
	s := strings.Join([]string{i.service, i.job, i.env, i.product}, ".")

	if i.zone != "" {
		s = strings.Join([]string{s, i.zone}, ".")
	}

	return s
}

type glimpseError struct {
	err error
	msg string
}

func newError(err error, format string, args ...interface{}) *glimpseError {
	return &glimpseError{
		err: err,
		msg: fmt.Sprintf(format, args...),
	}
}

func (e *glimpseError) Error() string {
	return fmt.Sprintf("%s: %s", e.err, e.msg)
}

func isConsulAPI(err error) bool {
	return unwrapError(err) == errConsulAPI
}

func isInvalidIP(err error) bool {
	return unwrapError(err) == errInvalidIP
}

func isNoInstances(err error) bool {
	return unwrapError(err) == errNoInstances
}

func unwrapError(err error) error {
	switch e := err.(type) {
	case *glimpseError:
		return e.err
	}

	return err
}

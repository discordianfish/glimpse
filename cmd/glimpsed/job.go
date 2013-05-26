package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strings"

	"code.google.com/p/goprotobuf/proto"
)

var label = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type validationErrors []string

func (v validationErrors) Error() string {
	return strings.Join(v, ", ")
}

func (v *validationErrors) Matches(re *regexp.Regexp, test string, name string) {
	if !re.MatchString(test) {
		*v = append(*v, fmt.Sprintf("%s:%q must match %q", name, test, re))
	}
}

func (v *validationErrors) Result() error {
	if v == nil || len(*v) == 0 {
		return nil
	}
	return *v
}

func (j *Job) Validate() error {
	v := new(validationErrors)

	v.Matches(label, j.GetZone(), "zone")
	v.Matches(label, j.GetProduct(), "product")
	v.Matches(label, j.GetEnv(), "env")
	v.Matches(label, j.GetName(), "name")

	return v.Result()
}

func DecodeJob(r io.Reader) (*Job, error) {
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("short request body: %v", err)
	}

	job := new(Job)
	if err := proto.Unmarshal(body, job); err != nil {
		return nil, fmt.Errorf("protobuf unmarshal error: %v", err)
	}

	return job, nil
}

func EncodeJob(w io.Writer, job *Job) error {
	body, err := proto.Marshal(job)
	if err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

func (j *Job) Path() Path {
	return Path(
		"/" + j.GetZone() +
			"/" + j.GetProduct() +
			"/" + j.GetEnv() +
			"/" + j.GetName())
}

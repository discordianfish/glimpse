package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"regexp"
	"sort"
	"strings"

	"code.google.com/p/goprotobuf/proto"
)

var label = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type ServiceAddress string

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

func (j *Job) Services() []Service {
	var sg ServiceGroup
	if j == nil {
		return []Service{}
	}

	for _, instance := range j.GetInstance() {
		for _, endpoint := range instance.GetEndpoint() {
			srv := Service{
				Job:      j,
				Instance: instance,
				Endpoint: endpoint,
			}
			sg = append(sg, srv)
		}
	}

	sort.Sort(sg)
	return sg
}

func DecodeJob(r io.Reader) (*Job, error) {
	body, err := ioutil.ReadAll(r)
	if len(body) == 0 || err != nil {
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

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	consul "github.com/hashicorp/consul/consul/structs"
	"github.com/soundcloud/visor"
)

func main() {
	var (
		chefJobs    = zoneMap{}
		doozerAddrs = zoneMap{}

		bazookaConfig = flag.String("bazooka.config", "./glimpse.json", "bazooka SD configuration")
		consulAddr    = flag.String("consul.addr", "127.0.0.1:8500", "consul http addr")
	)
	flag.Var(chefJobs, "chef.jobs", "chef zone and jobs list file tuple")
	flag.Var(doozerAddrs, "bazooka.doozer", "bazooka zone and doozer uri tuple")
	flag.Parse()

	c, err := readConfig(*bazookaConfig)
	if err != nil {
		log.Fatal(err)
	}

	statc := make(chan stat, len(chefJobs)+len(doozerAddrs))

	for zone, jobsFile := range chefJobs {
		go func(zone, jobsFile string) {
			stat, err := chefSync(zone, jobsFile, *consulAddr)
			if err != nil {
				log.Fatalf("chef sync for %s failed: %s", zone, err)
			}
			statc <- stat
		}(zone, jobsFile)
	}

	for zone, uri := range doozerAddrs {
		go func(zone, uri string) {
			stat, err := bazookaSync(zone, uri, *consulAddr, c)
			if err != nil {
				log.Fatalf("bzooka sync for %s failed: %s", zone, err)
			}
			statc <- stat
		}(zone, uri)
	}

	for i := 0; i < cap(statc); i++ {
		stat := <-statc

		log.Printf("[%s] %d %s services registered\n", stat.zone, stat.services, stat.scheduler)
	}

	select {}
}

func bazookaSync(zone, uri, consulAddr string, c config) (stat, error) {
	s := stat{
		scheduler: "bazooka",
		services:  0,
		zone:      zone,
	}

	store, err := visor.DialUri(uri, "/bazooka")
	if err != nil {
		return s, err
	}

	for app, procs := range c.Apps {
		a, err := store.GetApp(app)
		if err != nil {
			if visor.IsErrNotFound(err) {
				continue
			}
			return s, err
		}

		for proc, srv := range procs {
			p, err := a.GetProc(proc)
			if err != nil {
				if visor.IsErrNotFound(err) {
					continue
				}
				return s, err
			}

			is, err := p.GetInstances()
			if err != nil {
				return s, err
			}

			for _, ins := range is {
				if ins.Status != visor.InsStatusRunning {
					continue
				}

				err := catalogRegister(insToRegisterRequest(zone, ins, srv), consulAddr)
				if err != nil {
					return s, err
				}

				s.services++
			}
		}
	}
	return s, nil
}

func chefSync(zone, jobsFile, consulAddr string) (stat, error) {
	s := stat{
		scheduler: "chef",
		services:  0,
		zone:      zone,
	}

	jobs, err := readJobsFile(jobsFile)
	if err != nil {
		return s, err
	}

	for _, job := range jobs {
		err := catalogRegister(jobToRegisterRequest(zone, job), consulAddr)
		if err != nil {
			return s, err
		}

		s.services++
	}

	return s, nil
}

type job struct {
	srvInfo

	Port int    `json:"port"`
	IP   string `json:"ip"`
	Host string `json:"host"`
}

type config struct {
	Apps map[string]srvMap `json:"map"`
}

type srvInfo struct {
	Product string `json:"product"`
	Env     string `json:"env"`
	Job     string `json:"job"`
	Service string `json:"service"`
}

type srvMap map[string]srvInfo

type stat struct {
	scheduler string
	services  int
	zone      string
}

type zoneMap map[string]string

func (z zoneMap) Set(value string) error {
	split := strings.SplitN(value, "/", 2)

	z[split[0]] = split[1]

	return nil
}

func (z zoneMap) String() string {
	s := ""
	for zone, addr := range z {
		s = fmt.Sprintf("%s%s", s, fmt.Sprintf("%s/%s,", zone, addr))
	}
	return s
}

func catalogRegister(reg *consul.RegisterRequest, addr string) error {
	b, err := json.Marshal(reg)
	if err != nil {
		return err
	}

	uri := fmt.Sprintf("http://%s/v1/catalog/register", addr)
	req, err := http.NewRequest("PUT", uri, bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	res, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s register returned %s\n%+v", reg.Service.ID, res.Status, reg)
	}

	return nil
}

func insToRegisterRequest(zone string, ins *visor.Instance, srv srvInfo) *consul.RegisterRequest {
	tags := append(srvToTags(srv), "glimpse:bazooka")

	return &consul.RegisterRequest{
		// Datacenter: zone,
		Node:    ins.Host,
		Address: ins.Ip,
		Service: &consul.NodeService{
			ID:      ins.WorkerId(),
			Service: srv.Product,
			Tags:    tags,
			Port:    ins.Port,
		},
	}
}

func jobToID(j job) string {
	return fmt.Sprintf("%s-%s-%s-%s-%d", j.Product, j.Env, j.Job, j.Service, j.Port)
}

func jobToRegisterRequest(zone string, j job) *consul.RegisterRequest {
	tags := append(srvToTags(j.srvInfo), "glimpse:chef")

	return &consul.RegisterRequest{
		// Datacenter: zone,
		Node:    j.Host,
		Address: j.IP,
		Service: &consul.NodeService{
			ID:      jobToID(j),
			Service: j.Product,
			Tags:    tags,
			Port:    j.Port,
		},
	}
}

func srvToTags(srv srvInfo) []string {
	return []string{
		fmt.Sprintf("glimpse:env=%s", srv.Env),
		fmt.Sprintf("glimpse:job=%s", srv.Job),
		fmt.Sprintf("glimpse:product=%s", srv.Product),
		fmt.Sprintf("glimpse:service=%s", srv.Service),
	}
}

func readConfig(file string) (config, error) {
	c := config{}

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return c, err
	}

	err = json.Unmarshal(b, &c)
	if err != nil {
		return c, err
	}

	return c, nil
}

func readJobsFile(file string) ([]job, error) {
	jobs := []job{}

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &jobs)
	if err != nil {
		return nil, err
	}

	return jobs, nil
}

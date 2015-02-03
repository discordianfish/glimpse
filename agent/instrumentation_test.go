package main

import (
	"bytes"
	"net"
	"reflect"
	"testing"

	"github.com/miekg/dns"
)

func TestConsulCollectorParseAgent(t *testing.T) {
	var (
		output = `
agent:
        check_monitors = 0
        check_ttls = 0
        checks = 0
        services = 8
build:
        prerelease =
        revision = 461c1e18
        version = 0.4.2.soundcloud4
consul:
        known_servers = 3
        server = false
runtime:
        arch = amd64
        cpu_count = 1
        goroutines = 36
        max_procs = 16
        os = linux
        version = go1.3
serf_lan:
        event_queue = 0
        event_time = 55
        failed = 0
        intent_queue = 0
        left = 0
        member_time = 116
        members = 11
        query_queue = 0
        query_time = 1
`

		r = bytes.NewBufferString(output)
	)

	stats, err := parseConsulStats(r)
	if err != nil {
		t.Fatalf("parse failed: %s", err)
	}

	if want, got := 18, len(stats); want != got {
		t.Errorf("want %d, got %d", want, got)
	}

	want := consulStats{
		"agent_check_monitors":  0,
		"agent_check_ttls":      0,
		"agent_checks":          0,
		"agent_services":        8,
		"consul_known_servers":  3,
		"consul_server":         0,
		"runtime_cpu_count":     1,
		"runtime_goroutines":    36,
		"runtime_max_procs":     16,
		"serf_lan_event_queue":  0,
		"serf_lan_event_time":   55,
		"serf_lan_failed":       0,
		"serf_lan_intent_queue": 0,
		"serf_lan_left":         0,
		"serf_lan_member_time":  116,
		"serf_lan_members":      11,
		"serf_lan_query_queue":  0,
		"serf_lan_query_time":   1,
	}

	if !reflect.DeepEqual(want, stats) {
		t.Errorf("want:\n%v\ngot:\n%v", want, stats)
	}
}

func TestConsulCollectorParseServer(t *testing.T) {
	var (
		output = `
agent:
        check_monitors = 0
        check_ttls = 0
        checks = 0
        services = 4
build:
        prerelease =
        revision = 461c1e18
        version = 0.4.2.soundcloud4
consul:
        bootstrap = false
        known_datacenters = 1
        leader = true
        server = true
raft:
        applied_index = 1712470
        commit_index = 1712470
        fsm_pending = 0
        last_contact = never
        last_log_index = 1712470
        last_log_term = 131
        last_snapshot_index = 1708783
        last_snapshot_term = 131
        num_peers = 2
        state = Leader
        term = 131
runtime:
        arch = amd64
        cpu_count = 1
        goroutines = 81
        max_procs = 16
        os = linux
        version = go1.3
serf_lan:
        event_queue = 0
        event_time = 55
        failed = 0
        intent_queue = 0
        left = 0
        member_time = 116
        members = 11
        query_queue = 0
        query_time = 1
serf_wan:
        event_queue = 0
        event_time = 1
        failed = 0
        intent_queue = 0
        left = 0
        member_time = 1
        members = 1
        query_queue = 0
        query_time = 1
`
		r = bytes.NewBufferString(output)
	)

	stats, err := parseConsulStats(r)
	if err != nil {
		t.Fatalf("parse failed: %s", err)
	}

	if want, got := 39, len(stats); want != got {
		t.Errorf("want %d, got %d", want, got)
	}

	want := consulStats{
		"agent_check_monitors":     0,
		"agent_check_ttls":         0,
		"agent_checks":             0,
		"agent_services":           4,
		"consul_bootstrap":         0,
		"consul_known_datacenters": 1,
		"consul_leader":            1,
		"consul_server":            1,
		"raft_applied_index":       1712470,
		"raft_commit_index":        1712470,
		"raft_fsm_pending":         0,
		"raft_last_contact":        0,
		"raft_last_log_index":      1712470,
		"raft_last_log_term":       131,
		"raft_last_snapshot_index": 1708783,
		"raft_last_snapshot_term":  131,
		"raft_num_peers":           2,
		"raft_term":                131,
		"runtime_cpu_count":        1,
		"runtime_goroutines":       81,
		"runtime_max_procs":        16,
		"serf_lan_event_queue":     0,
		"serf_lan_event_time":      55,
		"serf_lan_failed":          0,
		"serf_lan_intent_queue":    0,
		"serf_lan_left":            0,
		"serf_lan_member_time":     116,
		"serf_lan_members":         11,
		"serf_lan_query_queue":     0,
		"serf_lan_query_time":      1,
		"serf_wan_event_queue":     0,
		"serf_wan_event_time":      1,
		"serf_wan_failed":          0,
		"serf_wan_intent_queue":    0,
		"serf_wan_left":            0,
		"serf_wan_member_time":     1,
		"serf_wan_members":         1,
		"serf_wan_query_queue":     0,
		"serf_wan_query_time":      1,
	}

	if !reflect.DeepEqual(want, stats) {
		t.Errorf("want:\n%v\ngot:\n%v", want, stats)
	}
}

func TestDnsMetricsHandler(t *testing.T) {
	testHandler := dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		res := &dns.Msg{}
		res.SetReply(req)
		res.SetRcode(req, dns.RcodeNotImplemented)

		err := w.WriteMsg(res)
		if err != nil {
			t.Fatalf("write response failed: %s", err)
		}
	})

	w := &testWriter{
		remoteAddr: &net.UDPAddr{},
	}

	m := &dns.Msg{}
	m.SetQuestion(dns.Fqdn("app.glimpse.io"), dns.TypeA)

	dnsMetricsHandler(testHandler).ServeDNS(w, m)
	r := w.msg

	if want, got := dns.RcodeNotImplemented, r.Rcode; want != got {
		t.Errorf("want rcode %s, got %s", dns.RcodeToString[want], dns.RcodeToString[got])
	}
}

func TestMetricsStoreGetInstances(t *testing.T) {
	var (
		i = info{
			service: "http",
			job:     "walker",
			env:     "prod",
			product: "harpoon",
			zone:    "tt",
		}
		ins = generateInstancesFromInfo(i)
		s   = newMetricsStore(&testStore{instances: map[info]instances{i: ins}})
	)

	sins, err := s.getInstances(i)
	if err != nil {
		t.Fatalf("want store to not return an error, got %s", err)
	}

	if want, got := ins, sins; !reflect.DeepEqual(want, got) {
		t.Errorf("want %d instances, got %d", len(want), len(got))
	}
}

func TestMetricsStoreGetServers(t *testing.T) {
	var (
		zone = "tt"
		srvs = instances{{host: "foo"}}
		m    = map[string]instances{zone: srvs}
		s    = newMetricsStore(&testStore{servers: m})
	)

	ss, err := s.getServers(zone)
	if err != nil {
		t.Fatalf("want store to not return an error, got %s", err)
	}
	if want, got := srvs, ss; !reflect.DeepEqual(want, got) {
		t.Errorf("want %d instances, got %d", len(want), len(got))
	}
}

// +build acceptance

package main

var expectedMetrics = []string{
	"glimpse_agent_dns_request_duration_microseconds_count",
	"glimpse_agent_consul_request_duration_microseconds_count",
	"glimpse_agent_consul_responses",
	"consul_raft_num_peers",
	"process_open_fds",
	"consul_process_open_fds",
}

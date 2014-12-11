# Glimpse

Glimpse is a **Service Discovery** platform build around srv tuples which provides service topology information hierarchically ordered by products and can optionally sliced into zones for isolation. Services can discover each other via the DNS or HTTP interface and decide based on the health information how to interact.

Glimpse tries to offer ease of integration from scheduling platforms by offering the concept of a provider. The surface for the provider is deliberately scoped on the host to limit the state both systems have to reason about to lowest common denominator.

The current implementation uses [Consul](https://www.consul.io/) as the backing service and the architecture of Glimpse itself is heavily influenced by it. Consul was chosen for it’s:

* consensus store for service information
* failure detection on host and service level
* functional HTTP API
* simple configuration

# API

## DNS

The DNS interface expects an `srv address` and offers:

- SRV: and A records and expects an srv address:
```
query:
<service>.<job>.<env>.<product>.<zone>.<dns_zone>.
answer:
All instances for service address scoped by zone
```

- A
```
query:
<service>.<job>.<env>.<product>.<zone>.<dns_zone>.
answer:
IPs of all instances for service address scoped by zone
```

The agent does not provide a fully implemented DNS server as it offers no recursion, no caching, no random/round-robin. For that reason we assume that the agent is fronted with an application like [Unbound](https://unbound.net/).

# Architecture

Every physical host will be provided with an agent accepting service configuration over well-defined interfaces as well as providing output interfaces (DNS, HTTP) for consumption of Service Discovery information. Agents will be stateless, except for health status and participation in the liveliness detection of other agents. All agents talk to the server ring which holds the data in a highly consistent way and can respond to requests. This should support the needs inside of a single zone. Cross zone requests will be accounted for by connecting rings in different zones in a similar fashion how agents are connected inside a zone. The server is responsible for forwarding of requests which concern other zones. Failure of servers will not affect operations as long as a quorum of servers is alive (N/2 + 1 where N is amount of servers building the ring). Complete failure of zones will not affect operations of any other zone.

## Overview

![Overview](http://i.imgur.com/SMZ4bPs.png)

## Current Host Interactions

Every provider atomically writes out a services configuration file representing the entire state known to him (scoped by the host). The write is followed by a reload of the consul-agent which is configured to read all configuration files from the directory known to the providers. Partial updates are avoided to simplify provider logic.
Unbound stays the main entry point for all DNS interaction on the host. It runs with a configured stub zone (sd.int.s-cloud.net) pointing to the local glimpse-agent which will answer all SRV and A queries by talking to the consul-agent HTTP API.

![Current Host Interactions](http://i.imgur.com/Z2LcDsR.png)

## Potential Host Interactions

Each provider calls the glimpse-agent HTTP endpoint described in the API section to update its list of known instances. When the call completes with success the caller can expect that the information is persisted and in propagation through out the infrastructure.
Unbound remains with the same set of responsibilities as described above. Additionally the agent will offer an HTTP interface with overlapping functionality to the DNS interface and potentially more complex query options. Please refer to the API section for more information.

![Potential Host Interactions](http://i.imgur.com/8NCrMyv.png)

## Development

Run `make setup` to install all necessary dependencies and pre-commit hooks.

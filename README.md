# Glimpse [![Build][1]][2] [![Coverage][3]][4]

[1]: https://circleci.com/gh/soundcloud/glimpse/tree/master.svg?style=svg
[2]: https://circleci.com/gh/soundcloud/glimpse/tree/master
[3]: https://img.shields.io/coveralls/soundcloud/glimpse.svg
[4]: https://coveralls.io/r/soundcloud/glimpse?branch=master

Glimpse is a **Service Discovery** platform, build around SRV tuples, which provides service topology information, hierarchically ordered by products, and can optionally sliced into zones for isolation. Services can discover each other via the DNS or HTTP interfaces, and decide based on the health information how to interact.

Glimpse tries to offer ease of integration by offering the concept of a **provider**. Providers are host-local entities which feed service information into Glimpse. The surface for the provider is deliberately scoped to the host, to limit the state both systems have to reason about.

The current implementation uses [Consul](https://www.consul.io/) as the backing service. The architecture of Glimpse itself is heavily influenced by it. Consul was chosen for its:

* consensus store for service information
* failure detection on host and service level
* functional HTTP API
* simple configuration

# API

## DNS

The DNS interface expects an `SRV address` and offers:

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

The agent does not provide a fully implemented DNS server, as it offers no recursion, no caching, and no random/round-robin behaviors. For that reason we assume that the agent is deployed behind a more fully-featured DNS server, like [Unbound](https://unbound.net/).

# Architecture

Every physical host in the infrastructure runs an **agent**, accepting service configuration over well-defined interfaces, as well as providing output interfaces (DNS, HTTP) for consumption of service discovery information. Agents will be stateless, except for health status, and participation in the liveliness detection of other agents. All agents talk to the **server ring**, which holds the complete topology in a highly consistent way, and can respond to requests.

This architecture supports the needs inside of a single zone. Cross-zone requests are handled by connecting rings in different zones, similarly to how agents are connected within a single zone. Servers are responsible for forwarding requests which concern other zones. Failure of servers will not affect operations as long as a quorum of servers is alive (N/2 + 1, where N is the number of servers comprising the ring). Complete failure of zones will not affect operations of any other zone.

## Overview

![Overview](http://i.imgur.com/elE3V3H.png)

## Current Host Interactions

Several components coexist on each individual host in the infrastructure.

![Host components](http://i.imgur.com/1Eb0QSb.png)

To update the topology, every provider atomically writes out a services configuration file, representing the entire state known to it, scoped to the host. The write is followed by a reload of the local consul-agent, which is configured to read all configuration files from the directory known to the providers. Partial (delta) updates are avoided, to simplify provider logic. The configuration (topology) information is propegated to the rest of the infrastructure through the server ring.

![Config flow](http://i.imgur.com/3ohBadj.png)

To service requests, unbound remains the main entry point for all DNS interaction. It runs with a configured stub zone pointing to the local glimpse-agent, which will answer all SRV and A queries by talking to the consul-agent HTTP API. Requests flow through components on a single host, and generally escaping the host to reach the zone-local server ring.

![Request flow](http://i.imgur.com/lHBZQQj.png)

Responses follow the same path back to the client.

![Response flow](http://i.imgur.com/NTVFPUP.png)

## Future Host Interactions

In the future, each provider will be able to call the glimpse-agent HTTP API (described in the [API section](#api)) to update known instances. When the call completes successfully, the caller can expect the information is persisted, and being propegated throughout the global infrastructure.

Unbound keeps the same responsibilities described above. Additionally, the glimpse-agent will offer an HTTP interface with the same functionality as the DNS interface, and potentially more complex query options, including subscription semantics for passive discovery. Please refer to the [API section](#api) for more information.

![Future host interactions](http://i.imgur.com/YRHA8EG.png)

## Development

Run `make setup` to install all necessary dependencies and pre-commit hooks.

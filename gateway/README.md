# Gateway Agent

Linux gateway agent for applying a validated desired network state. The control plane remains responsible for identity, authorization, and policy decisions; the gateway validates and applies the resulting tunnel, routes, NAT mappings, and firewall rules.

## Current functionality

- `config` strictly decodes versioned JSON, validates WireGuard keys, interface names, peers, routes, MTU, DNAT/SNAT mappings, and ACL rule structure, and canonicalizes prefixes. Gateway `routes` must be covered by a configured Peer `allowed_ips`; exact duplicate AllowedIPs across Peers are rejected.
- `gateway-agent keygen --dir` creates one WireGuard key pair in a mode `0700` directory, writes the private key as `0600`, and refuses to overwrite existing files. The WireGuard public key is also the node's network identity; output contains only that public key.
- Config files contain or reference private key material. `gateway-agent validate` requires a regular, non-symlink config file and referenced key file with no group or other permissions, and prints only a redacted summary.
- `internal/agent` reconciles complete generations idempotently. It blocks forwarding first, applies tunnel and firewall state before routes and NAT, enables forwarding last, and reports ready only after the full sequence succeeds.

Linux NAT supports the existing prefix DNAT form and fixed-address SNAT. SNAT rules are restricted by tunnel source prefix, destination prefix, and an explicit egress interface; the translation address must already be assigned to that interface. Access authorization remains in the forwarding ACL and uses the original packet source address.
- `Reconciler.Start` blocks forwarding while the agent waits for its first trusted config generation.
- Invalid config, a conflicting generation, or a dataplane apply failure triggers a fail-closed block and dataplane close. Empty rule sets mean default deny. Stale generations are rejected without changing a healthy active generation.
- `HealthHandler` reports ready only after a complete generation has been applied and never returns key material or backend error details.
- `platform/linux` uses kernel WireGuard with `wgctrl`, netlink routes, and isolated nftables tables. It falls back to WireGuard-Go only when kernel WireGuard support is unavailable; permission and configuration failures remain errors.
- `tunnel.mtu` is applied to both kernel and userspace WireGuard interfaces. Its accepted range is 1280–9000 bytes; omission retains the previous 1420-byte default.
- When Docker's iptables `FORWARD` path is present, `platform/linux` mirrors allowed ACL tuples and established replies into a per-interface chain linked from `DOCKER-USER`. It preserves preceding host rules and Docker's `FORWARD` policy, checks the integration every 15 seconds for Docker chain recreation, removes the owned rules while forwarding is blocked or the agent shuts down, and leaves VeloTrust's nftables ACL as the authorization authority. This requires the matching `iptables`/`ip6tables` frontend and Docker's linked `DOCKER-USER` chain.
- `gateway-agent run --log-level debug` logs reconciliation stages, sanitized peer key fingerprints, and kernel WireGuard handshake counters. Private and preshared keys are not logged.
- `gateway-agent run` watches its private local config file for complete replacement generations. It blocks and closes the dataplane when the file becomes invalid or unreadable, then reapplies a repaired valid generation automatically. The agent does not yet connect to the management control plane.

Example preflight:

```sh
gateway-agent keygen --dir /etc/velotrust/gateway
gateway-agent validate --config /etc/velotrust/gateway/gateway.json
```

Key generation writes `wireguard.key` and `wireguard.pub`. In the config, set `tunnel.private_key_file` to `wireguard.key`; the filename must stay in the same private directory as the config. The loader checks file permissions and includes the key contents in its reload fingerprint. Keep the directory private and share only the public key. The control plane must authenticate a node during enrollment and bind its verified node record to this WireGuard public key before authorizing or distributing peer configuration. The current static-config agent does not yet perform that enrollment. Existing `node-identity.*` files from earlier versions are unused and are not removed automatically.

Start the Linux agent with:

```sh
gateway-agent run --config /etc/velotrust/gateway/gateway.json --health-listen 127.0.0.1:8765
```

The service needs permission to create/configure the tunnel, routes, nftables rules, and forwarding sysctls. The health listener defaults to loopback. Run with an empty ACL to keep forwarding denied. The reconciler remains fail-closed when a config or dataplane step fails.

Replace the config atomically with a new, higher generation to avoid exposing partial writes to the watcher. The current implementation has not been validated with privileged end-to-end packet forwarding in this development environment. Forced process termination can leave the last kernel tunnel and firewall state active until service recovery; config leases and control-plane synchronization are future work. See [the network config contract](../docs/VeloTrust-网络配置契约.md) for the shared Client/Gateway field meanings and [the gateway implementation plan](../docs/VeloTrust-网关节点技术实现方案.md) for backend behavior and current limitations.

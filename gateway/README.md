# Gateway Agent

Linux gateway agent for applying a validated desired network state. The control plane remains responsible for identity, authorization, and policy decisions; the gateway validates and applies the resulting tunnel, routes, NAT mappings, and firewall rules.

## Current functionality

- `config` strictly decodes versioned JSON, validates WireGuard keys, interface names, peers, associated networks, MTU, DNAT/SNAT/masquerade mappings, and ACL rule structure, and canonicalizes prefixes. Gateway `routes` are networks behind the Gateway; the Linux driver resolves each network against the existing host routing table and rejects paths that recurse into the managed WireGuard interface. It does not install duplicate site routes; the host's connected, static, or default route selects the egress. Exact duplicate AllowedIPs across Peers are rejected.
- `gateway-agent keygen --dir` creates one WireGuard key pair in a mode `0700` directory, writes the private key as `0600`, and refuses to overwrite existing files. The WireGuard public key is also the node's network identity; output contains only that public key.
- Config files contain or reference private key material. `gateway-agent validate` requires a regular, non-symlink config file and referenced key file with no group or other permissions, and prints only a redacted summary.
- `internal/agent` reconciles complete config snapshots idempotently by content digest. It blocks forwarding first, applies the tunnel and firewall, installs Peer `AllowedIPs` routes over WireGuard, validates that associated site networks resolve through an existing non-tunnel host route, applies NAT, then enables forwarding and reports ready.

Linux NAT supports prefix DNAT, fixed-address SNAT, and source/destination-scoped masquerade. HUB automatically generates a masquerade rule from the global Overlay range to each active network associated with the Gateway; optional site shadow mappings add DNAT while retaining that automatic SNAT. Fixed SNAT rules require an explicit egress interface and an address assigned to that interface. Access authorization remains in the forwarding ACL and uses the original packet source address.
- `Reconciler.Start` blocks forwarding while the agent waits for its first valid static config or HUB snapshot.
- Invalid config or a dataplane apply failure triggers a fail-closed block and dataplane close. Empty rule sets mean default deny. A changed, valid local config replaces the active snapshot.
- `HealthHandler` reports ready only after a complete config snapshot has been applied and never returns key material or backend error details.
- `platform/linux` uses kernel WireGuard with `wgctrl`, netlink routes, and isolated nftables tables. It falls back to WireGuard-Go only when kernel WireGuard support is unavailable; permission and configuration failures remain errors.
- `tunnel.mtu` is applied to both kernel and userspace WireGuard interfaces. Its accepted range is 1280–9000 bytes; omission retains the previous 1420-byte default.
- When Docker's iptables `FORWARD` path is present, `platform/linux` mirrors allowed ACL tuples and established replies into a per-interface chain linked from `DOCKER-USER`. It preserves preceding host rules and Docker's `FORWARD` policy, checks the integration every 15 seconds for Docker chain recreation, removes the owned rules while forwarding is blocked or the agent shuts down, and leaves VeloTrust's nftables ACL as the authorization authority. This requires the matching `iptables`/`ip6tables` frontend and Docker's linked `DOCKER-USER` chain.
- `gateway-agent run --log-level debug` logs reconciliation stages, sanitized peer key fingerprints, and kernel WireGuard handshake counters. Private and preshared keys are not logged.
- `gateway-agent run` watches its private local config file for complete replacements. It blocks and closes the dataplane when the file becomes invalid or unreadable, then reapplies a repaired valid config automatically. Legacy `generation` fields are accepted and ignored. HUB connectivity is direct; the Gateway does not call the Management API.

When a `hub` section is present in the Gateway config, the agent opens a HUB gRPC control stream. TLS is enabled by default; `hub.tls: false` explicitly selects plaintext transport. The agent authenticates with its WireGuard X25519 key; HUB resolves an enrolled node from that public key. If the key is not registered, the agent submits `hub.registration_token`, which HUB uses to resolve the pre-created node. The X25519 challenge carries the resolved node code and binds it into the proof. If Management approval is required, the HUB registration stream remains open and the process waits until approval; after approval the agent repeats normal identity authentication. Forwarding stays blocked until identity authentication succeeds. The local config no longer requires `gateway_id`; existing `gateway_id` values are ignored for compatibility. Plaintext mode does not encrypt or protect control messages from modification and does not authenticate the HUB server; use it only on a trusted isolated network for development. Status reports reflect the local configuration reconciler state; they do not prove a recent WireGuard handshake or end-to-end reachability. Exact connected-peer counts are currently unavailable and are explicitly marked unknown. The agent reconnects with bounded backoff and restarts the control stream when a valid config replacement changes the HUB endpoint or identity key. HUB sends a content-hashed snapshot compiled from IPAM leases, active tunnel peers, associated networks, and NAT rows; Gateway receipts update the delivery task state. `rules` currently compiles empty and therefore keeps forwarding default-deny until a policy compiler is defined.

Example HUB client settings:

```json
{
  "hub": {
    "endpoint": "hub.example.com:8443",
    "server_name": "hub.example.com",
    "ca_file": "/etc/velotrust/hub-ca.pem",
    "tls": true,
    "registration_token": "vtreg_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
  }
}
```

The sample token is a format-only placeholder and must be replaced with the one-time credential issued for this pre-created node. `registration_token` is optional for an already registered Gateway. The private config file must remain mode `0600`; the agent clears the active config field after passing it to the control client, and the control client clears its credential buffer after HUB accepts it. The agent does not rewrite the config file, so remove the consumed value from it after registration.

`ca_file` is optional when the HUB certificate chains to a system trust root. When configured, it is the explicit CA trust bundle; a relative path is resolved from the Gateway config directory. Gateway TLS defaults to enabled; with TLS enabled, certificate and hostname verification remain enabled. HUB TLS is optional and defaults to enabled when certificate paths are configured; set `tls.enabled` to `false` (or omit the TLS block) for plaintext gRPC. To connect in plaintext mode, also set `hub.tls` to `false` in the Gateway config. Both ends must use the same transport mode. Plaintext does not encrypt or authenticate the HUB server and is intended only for isolated development networks.

Example preflight:

```sh
gateway-agent keygen --dir /etc/velotrust/gateway
gateway-agent validate --config /etc/velotrust/gateway/gateway.json
```

Key generation writes `wireguard.key` and `wireguard.pub`. In the config, set `tunnel.private_key_file` to `wireguard.key`; the filename must stay in the same private directory as the config. The loader checks file permissions and includes the key contents in its reload fingerprint. Keep the directory private and share only the public key. On first `run`, the agent authenticates first, then proves possession of this WireGuard key while submitting the configured one-time token. If Management approval is required, `run` stays blocked with forwarding disabled until approval; it then authenticates again before applying the dataplane. After authentication, the agent receives configuration compiled by HUB from IPAM leases, tunnel peers, network-site associations, and NAT records. Existing `node-identity.*` files from earlier versions are unused and are not removed automatically.

Start the Linux agent with:

```sh
gateway-agent run --config /etc/velotrust/gateway/gateway.json --health-listen 127.0.0.1:8765
```

The service needs permission to create/configure the tunnel, routes, nftables rules, and forwarding sysctls. The health listener defaults to loopback. In HUB mode, Management's active access-policy rules are compiled into Gateway ACL rules; an empty ACL still means deny all forwarded traffic. The reconciler remains fail-closed when a config or dataplane step fails.

Replace the config atomically to avoid exposing partial writes to the watcher. The Agent detects changes from the config content and private-key file digest; no manual revision field is required. The current implementation has not been validated with privileged end-to-end packet forwarding in this development environment. Forced process termination can leave the last kernel tunnel and firewall state active until service recovery; config leases are not implemented. See [the network config contract](../docs/VeloTrust-网络配置契约.md) for the shared Client/Gateway field meanings and [the gateway implementation plan](../docs/VeloTrust-网关节点技术实现方案.md) for backend behavior and current limitations.

# Deferred network modules and integration boundaries

The inspected source is pinned to `v0.80.0`, commit `fca64287cf51a85552a41b022013abbfdb335452`, from the supplied `netbird-0.80.0.zip`. License was not used as an exclusion criterion for this integration pass. Modules below remain out because they are outside the current client networking scope, depend on unavailable packages, or need platform and privileged runtime validation that this environment cannot provide.

The file-level integration inventory, including copied source paths and modifications, is in [netbird-reuse-manifest.md](netbird-reuse-manifest.md).

## Project services outside the networking integration

| Upstream path | Status | Reason |
| --- | --- | --- |
| `management/` | Not integrated | VeloTrust retains its own identity, device trust, resource, policy, persistence, and control-plane implementation. Upstream management entities and APIs cannot replace those business boundaries. |
| `signal/` | Not integrated | This is a separately deployed signaling service. The current task integrates client-side network primitives and does not establish VeloTrust signaling contracts or deployment behavior. |
| `relay/` | Not integrated | Relay service behavior requires VeloTrust protocol, lifecycle, and deployment decisions. Client ICE/relay coordination is likewise deferred until those contracts exist. |
| `combined/` | Not integrated | This bundles unrelated service entry points and does not provide an independently reusable network primitive for the current client. |

These are scope decisions, not license-based exclusions. No files from these directories are currently imported by VeloTrust Go packages.

## Network packages partially integrated or deferred

| Upstream path | Status | Integrated functionality | Remaining work or blocker |
| --- | --- | --- | --- |
| `dns/` | Partial | Generic record formatting, domain normalization, and resolver URL helpers. | Agent DNS listeners, resolver configuration, host file changes, and platform resolver lifecycle remain unconnected to VeloTrust's config sync. |
| `stun/` | Deferred | STUN packet recognition is available through the shared-socket BPF filter. | The bundled STUN server depends on a third-party STUN package and service lifecycle. Client NAT discovery belongs to Peer ICE coordination, whose transport dependencies are unavailable. |
| `route/` | Partial | Fake-IP allocation, HA identifiers, IP-to-prefix conversion, prefix ownership counters, and AllowedIPs ownership helpers. | The route entity contains service configuration. Route installation/removal needs VeloTrust desired-state inputs and OS-specific backends. |
| `sharedsock/` | Partial | BPF filter generation and Linux source-address route probing. | The raw shared UDP socket implementation also needs packet encode/decode, dual-stack socket lifecycle, raw-socket privileges, and runtime tests. Socket creation is denied in this environment (`EPERM`), so that backend is not claimed as complete. |
| `client/iface/` | Partial | MTU validation, address helpers, and allowed-prefix configuration bookkeeping. | WireGuard TUN/device creation, UAPI configuration, and platform interface lifecycle depend on WireGuard device, bind, and TUN modules unavailable in the local module cache. Network access is blocked, so those dependencies could not be fetched. |
| `client/firewall/` | Deferred | VeloTrust's separation between policy decisions and local rule application remains in place. | Linux, macOS, and Windows backends make privileged host changes and require platform-specific dependencies and fail-closed lifecycle integration. No firewall backend is reported as integrated. |
| `client/internal/peer/` | Partial | Transport priority, session ID helpers, and an independent fail-closed connection status classifier. | ICE negotiation, relay, signaling, peer state coordination, and WireGuard application depend on protocol and transport packages not present in VeloTrust. |
| `client/internal/routemanager/` | Partial | Fake-IP, route counters, sysctl setup, and route helpers. | Full reconciliation depends on the upstream route model, peer manager, DNS, firewall, and OS-specific route system operations. |
| `client/internal/networkmonitor/` | Deferred | VeloTrust retains its standalone online/offline state primitive. | OS route-change monitors depend on the deferred platform route backends and require validation on each target OS. |
| `client/net/` | Partial | Linux fwmark constants and marked-socket support were adapted to VeloTrust names. | The upstream bound dialer/listener and interface selection need integration with VeloTrust's Agent lifecycle and policy routing. |
| `shared/` | Partial | Compact IP/prefix encoding is integrated. | Other shared protocol code is deferred until VeloTrust defines the corresponding protocol contracts; management database/API types are not imported. |

## Known runtime gaps

The current working tree still does not create or configure a WireGuard device, install or remove routes, apply firewall policy, establish ICE/relay peer sessions, or bind the reusable raw shared socket. The Linux sysctl and fwmark helpers are available as primitives but are not called by an Agent lifecycle. These gaps require either the unavailable dependency modules, VeloTrust integration points, or host privileges for meaningful runtime verification.

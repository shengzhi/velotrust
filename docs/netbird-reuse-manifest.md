# BSD network code reuse manifest

## Pinned source

- Repository: [netbirdio/netbird](https://github.com/netbirdio/netbird)
- Release: [`v0.80.0`](https://github.com/netbirdio/netbird/releases/tag/v0.80.0)
- Commit: `fca64287cf51a85552a41b022013abbfdb335452`
- Source archive: `netbird-0.80.0.zip`
- Archive SHA-256: `986e324224258b8fc77aa1c7643e4c51a14f4ae9d70f049fa681f1eebe96ddcd`
- License: the upstream root license applies BSD-3-Clause except `management/`, `signal/`, `relay/`, and `combined/`, which are AGPL-3.0. No code or imports from those directories were copied.

The complete BSD-3-Clause notice, copyright attribution, conditions, and disclaimer are retained in `LICENSES/NetBird-BSD-3-Clause.txt`. Each reused Go file has an SPDX marker. Tests authored in this repository are identified as VeloTrust tests and are not counted as upstream source files.

## Reused source files

| Original source path | VeloTrust path | Release / commit | License | Adaptation | Added dependency | Test result |
| --- | --- | --- | --- | --- | --- | --- |
| `dns/dns.go` | `client/internal/dns/config.go` | `v0.80.0` / `fca64287cf51a85552a41b022013abbfdb335452` | BSD-3-Clause | Reused record formatting/length, domain label conversion, and wildcard normalization helpers. VeloTrust-owned config types replace upstream service and persistence fields. DNS library calls were replaced with local helpers. | Existing `golang.org/x/net v0.46.0` for IDNA. | Passed (`go test ./...`, Linux/amd64). |
| `dns/nameserver.go` | `client/internal/dns/config.go` | same | BSD-3-Clause | Reused the UDP resolver URL parsing shape as `ParseResolverURL`; validates literal IPs and ports and returns VeloTrust's `netip.AddrPort` type. Group/account model code was not copied. | None. | Passed (`go test ./...`, Linux/amd64). |
| `client/iface/iface.go` | `client/iface/mtu.go` | same | BSD-3-Clause | Reused standalone MTU bounds and validation only; interface lifecycle and platform backends were not copied. | None. | Passed (`go test ./...`, Linux/amd64). |
| `client/iface/wgaddr/address.go` | `client/iface/wgaddr/address.go` | same | BSD-3-Clause | Reused address and dual-stack prefix helpers, adding a local compact-prefix import. Retained VeloTrust's IPv4 primary-address validation and convenience constructor. | Local `velotrust/shared/netiputil`. | Passed (`go test ./...`, Linux/amd64). |
| `shared/netiputil/compact.go` | `shared/netiputil/compact.go` | same | BSD-3-Clause | Reused compact address/prefix encoding; added invalid-prefix validation. Standard library only. | None. | Passed (`go test ./...`, Linux/amd64). |
| `client/internal/routemanager/fakeip/fakeip.go` | `client/internal/route/fakeip/fakeip.go` | same | BSD-3-Clause | Reused dual-stack fake address allocation and reverse lookup. No route database or control-plane types were copied. | None. | Passed (`go test ./...`, Linux/amd64). |
| `sharedsock/filter.go` | `client/iface/sharedsock/filter.go` | same | BSD-3-Clause | Reused the generic BPF filter interface. | Existing `golang.org/x/net v0.46.0` BPF package. | Passed (`go test ./...`, Linux/amd64). |
| `sharedsock/stun_filter_linux.go` | `client/iface/sharedsock/stun_filter_linux.go` | same | BSD-3-Clause | Reused Linux STUN packet filter instruction generation. Original `_linux.go` platform selection is retained. | Existing `golang.org/x/net v0.46.0` BPF package. | Passed (`go test ./...`, Linux/amd64); Windows/macOS compile passed. |
| `sharedsock/stun_filter_nolinux.go` | `client/iface/sharedsock/stun_filter_nolinux.go` | same | BSD-3-Clause | Reused the non-Linux no-op filter, preserving filename-based platform selection. | None. | Passed (`go test ./...`, Linux/amd64); Windows/macOS compile passed. |
| `client/netevents/netstate/netstate.go` | `client/internal/network/netstate/state.go` | same | BSD-3-Clause | Reused the synchronized online/offline state and context-aware wait behavior. Replaced third-party logging with `log/slog` and removed references to upstream service consumers. | None. | Passed (`go test ./...`, Linux/amd64). |
| `client/internal/routemanager/refcounter/refcounter.go` | `client/internal/route/refcounter/refcounter.go` | same | BSD-3-Clause | Reused generic key reference counting, ID tracking, serialization, and reapply behavior. Replaced external multi-error/logging helpers with `errors.Join` and `log/slog`; fixed nil-map initialization during JSON restore. | None. | Passed (`go test ./...`, Linux/amd64). |
| `client/internal/routemanager/refcounter/allowedips.go` | `client/internal/route/refcounter/allowedips.go` | same | BSD-3-Clause | Reused peer-aware AllowedIPs reference tracking and deterministic peer handover. Replaced external error aggregation with `errors.Join`. | None. | Passed (`go test ./...`, Linux/amd64). |
| `client/internal/routemanager/refcounter/types.go` | `client/internal/route/refcounter/types.go` | same | BSD-3-Clause | Reused route counter type aliases and documentation. | None. | Passed (`go test ./...`, Linux/amd64). |
| `client/iface/configurer/allowedips.go` | `client/iface/configurer/allowedips.go` | same | BSD-3-Clause | Reused allowed-prefix ownership bookkeeping and v4-mapped prefix normalization. Replaced the WireGuard-specific key dependency with a comparable generic key type so the helper has no third-party dependency. | None. | Passed (`go test ./...`, Linux/amd64); Windows/macOS compile passed. |
| `route/route.go` (`NetID` type only) | `client/internal/route/hauniqueid.go` | same | BSD-3-Clause | Reused the small network identifier type required by the HA identifier helper. The persisted route entity was not copied. | None. | Passed (`go test ./...`, Linux/amd64). |
| `route/hauniqueid.go` | `client/internal/route/hauniqueid.go` | same | BSD-3-Clause | Reused HA route identifier parsing; kept in a VeloTrust-owned route utility package. | None. | Passed (`go test ./...`, Linux/amd64). |
| `client/internal/routemanager/sysctl/sysctl_linux.go` | `client/system/linux/sysctl/sysctl_linux.go` | same | BSD-3-Clause | Adapted idempotent sysctl updates, reverse-path filtering, source validation marks, and restoration. Logging uses `log/slog`; host interface names are escaped locally. | None. | Passed Linux tests; Windows/macOS compile passed. |
| `client/net/fwmark.go` | `client/internal/network/fwmark/marks.go` | same | BSD-3-Clause | Adapted mark allocation and validation; changed the environment variable to a VeloTrust-owned name. | None. | Passed Linux tests; Windows/macOS compile passed. |
| `client/net/net_linux.go` | `client/internal/network/fwmark/socket_linux.go` | same | BSD-3-Clause | Adapted Linux `SO_MARK` application for `syscall.Conn`; non-Linux behavior returns an explicit unsupported error. | `golang.org/x/sys v0.37.0` is used by the shared-socket probe, not this file. | Passed Linux build; Windows/macOS compile passed. |
| `client/internal/peer/conntype/priority.go` | `client/internal/peer/transport.go` | same | BSD-3-Clause | Adapted transport priority values and added a VeloTrust-owned connection status classifier from the independent decision logic in `client/internal/peer/conn.go`. Unknown transport state remains disconnected. | None. | Passed (`go test ./...`, Linux/amd64); Windows/macOS compile passed. |
| `client/internal/peer/conn.go` (`evalConnStatus` only) | `client/internal/peer/transport.go` | same | BSD-3-Clause | Adapted the pure transport-state decision function. The Worker, Handshaker, signaling, relay, and protocol code in the same source file was not copied. | None. | Passed (`go test ./...`, Linux/amd64); Windows/macOS compile passed. |
| `client/internal/peer/session_id.go` | `client/internal/peer/session_id.go` | same | BSD-3-Clause | Adapted random session ID generation, byte conversion, and strict validation. | None. | Passed (`go test ./...`, Linux/amd64); Windows/macOS compile passed. |
| `client/internal/routemanager/util/ip.go` | `client/internal/route/ip.go` | same | BSD-3-Clause | Adapted IP-to-host-prefix conversion, normalizing IPv4-mapped IPv6 input. | None. | Passed (`go test ./...`, Linux/amd64); Windows/macOS compile passed. |
| `client/internal/routemanager/notifier/route_diff.go` | `client/internal/route/prefixset.go` | same | BSD-3-Clause | Adapted route-set equality for VeloTrust `netip.Prefix` values; deduplicates and masks prefixes, and returns errors for invalid input. | None. | Passed (`go test ./...`, Linux/amd64); Windows/macOS compile passed. |
| `sharedsock/src_probe_linux.go` | `client/iface/sharedsock/probe_linux.go` | same | BSD-3-Clause | Adapted Linux source-address route probing. Replaced the third-party socket wrapper with direct `x/sys/unix` calls, exposed a small API, and made the routing mark optional. | `golang.org/x/sys v0.37.0`. | Linux compile passed. Runtime tests were skipped because the sandbox rejects socket creation with `EPERM`; Windows/macOS compile passed using the unsupported-platform adapter. |
| Upstream root `LICENSE` | `LICENSES/NetBird-BSD-3-Clause.txt` | same | BSD-3-Clause | Preserved copyright, all three redistribution conditions, and the full disclaimer; added the repository's AGPL exception scope and VeloTrust reuse inventory. | None. | Compared with the archive's root license. |

## Dependency and import audit

- No NetBird Go module dependency is present in `go.mod` or `go.sum`.
- Reused code imports only VeloTrust packages, Go standard library packages, and `golang.org/x/net` / `golang.org/x/sys`.
- No imported package resolves to an upstream AGPL directory.
- This integration pass added `golang.org/x/sys v0.37.0` as a direct dependency for Linux socket and sysctl operations; its BSD-3-Clause license copy already exists in `LICENSES/`.
- `golang.org/x/net` and its license copy predate this pass and are used by DNS IDNA handling and BPF instruction generation.

| Module in the existing graph | Use | License | License copy |
| --- | --- | --- | --- |
| `golang.org/x/net v0.46.0` | Direct import for IDNA and BPF | BSD-3-Clause | `LICENSES/golang.org-x-net-BSD-3-Clause.txt` |
| `golang.org/x/sys v0.37.0` | Direct import for Linux socket operations | BSD-3-Clause | `LICENSES/golang.org-x-sys-BSD-3-Clause.txt` |
| `golang.org/x/text v0.30.0` | Existing transitive dependency | BSD-3-Clause | `LICENSES/golang.org-x-text-BSD-3-Clause.txt` |
| `golang.org/x/crypto`, `x/mod`, `x/sync`, `x/sys`, `x/term`, `x/tools` | Module graph only; no reused code import | BSD-3-Clause | Corresponding files in `LICENSES/` |

## Validation results

- `gofmt -l` over all Go files under `client/` and `shared/`: passed; no files reported.
- `GOCACHE=/tmp/velotrust-go-cache GOPROXY=off GOSUMDB=off go mod tidy`: passed; added the required direct `golang.org/x/sys v0.37.0` dependency.
- `GOCACHE=/tmp/velotrust-go-cache GOPROXY=off GOSUMDB=off go test ./...`: passed on Linux/amd64 across 13 packages. The three socket-backed source-probe tests skipped because this environment denies socket creation (`EPERM`).
- `go list -deps ./...`: passed. The dependency list contains no upstream repository package or AGPL directory import.
- `go list -m all`: passed; only VeloTrust and existing `golang.org/x/*` modules are in the graph.
- Source scan of all Go files, `go.mod`, and `go.sum` for the upstream project name and module path: no matches.
- `GOOS=windows GOARCH=amd64 go test -c` and `GOOS=darwin GOARCH=amd64 go test -c`: compiled all 13 packages. These are compile-only checks; binaries were not run on those operating systems.

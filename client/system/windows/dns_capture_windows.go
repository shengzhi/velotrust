//go:build windows

// SPDX-License-Identifier: BSD-3-Clause

package windows

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const systemDNSMarkerPrefix = "VeloTrust DNS Proxy: "

type dnsCaptureRequest struct {
	Marker       string   `json:"marker"`
	ProxyServers []string `json:"proxy_servers,omitempty"`
	ProbeName    string   `json:"probe_name,omitempty"`
}

// DiscoverSystemDNS returns active adapters' configured DNS servers. Loopback
// resolvers are omitted to prevent forwarding a request back into this proxy.
func DiscoverSystemDNS() ([]netip.AddrPort, error) {
	var size uint32
	err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, 0, 0, nil, &size)
	if err != windows.ERROR_BUFFER_OVERFLOW && err != nil {
		return nil, fmt.Errorf("query active network adapters: %w", err)
	}
	if size == 0 {
		return nil, nil
	}
	buffer := make([]byte, size)
	first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0]))
	err = windows.GetAdaptersAddresses(windows.AF_UNSPEC, 0, 0, first, &size)
	if err != nil {
		return nil, fmt.Errorf("read active network adapters: %w", err)
	}
	runtime.KeepAlive(buffer)

	seen := make(map[netip.AddrPort]struct{})
	var result []netip.AddrPort
	for adapter := first; adapter != nil; adapter = adapter.Next {
		if adapter.OperStatus != windows.IfOperStatusUp || adapter.IfType == windows.IF_TYPE_SOFTWARE_LOOPBACK {
			continue
		}
		for server := adapter.FirstDnsServerAddress; server != nil; server = server.Next {
			endpoint, ok := dnsSocketAddress(server.Address, adapter.Ipv6IfIndex)
			if !ok || !validSystemResolver(endpoint) {
				continue
			}
			if _, exists := seen[endpoint]; exists {
				continue
			}
			seen[endpoint] = struct{}{}
			result = append(result, endpoint)
		}
	}
	return result, nil
}

func dnsSocketAddress(address windows.SocketAddress, ipv6IfIndex uint32) (netip.AddrPort, bool) {
	if address.Sockaddr == nil {
		return netip.AddrPort{}, false
	}
	switch address.Sockaddr.Addr.Family {
	case windows.AF_INET:
		if uintptr(address.SockaddrLength) < unsafe.Sizeof(windows.RawSockaddrInet4{}) {
			return netip.AddrPort{}, false
		}
		socket := (*windows.RawSockaddrInet4)(unsafe.Pointer(address.Sockaddr))
		ip := netip.AddrFrom4(socket.Addr)
		port := byteOrderPort(unsafe.Pointer(&socket.Port))
		if port == 0 {
			port = 53
		}
		return netip.AddrPortFrom(ip, port), true
	case windows.AF_INET6:
		if uintptr(address.SockaddrLength) < unsafe.Sizeof(windows.RawSockaddrInet6{}) {
			return netip.AddrPort{}, false
		}
		socket := (*windows.RawSockaddrInet6)(unsafe.Pointer(address.Sockaddr))
		ip := netip.AddrFrom16(socket.Addr)
		if ip.IsLinkLocalUnicast() {
			scopeID := socket.Scope_id
			if scopeID == 0 {
				scopeID = ipv6IfIndex
			}
			if scopeID != 0 {
				ip = ip.WithZone(strconv.FormatUint(uint64(scopeID), 10))
			}
		}
		port := byteOrderPort(unsafe.Pointer(&socket.Port))
		if port == 0 {
			port = 53
		}
		return netip.AddrPortFrom(ip, port), true
	default:
		return netip.AddrPort{}, false
	}
}

func byteOrderPort(value unsafe.Pointer) uint16 {
	return binary.BigEndian.Uint16(unsafe.Slice((*byte)(value), 2))
}

func validSystemResolver(endpoint netip.AddrPort) bool {
	address := endpoint.Addr()
	return endpoint.IsValid() && endpoint.Port() != 0 && !address.IsUnspecified() && !address.IsMulticast() && !address.IsLoopback() && !address.Is4In6()
}

// ApplyDNSCapture creates a local NRPT rule that directs normal Windows DNS
// lookups to the loopback proxy. The probe verifies that the effective system
// resolver path reaches the proxy before the tunnel is reported as ready.
func ApplyDNSCapture(instanceID string, probeName string) (func() error, error) {
	marker := systemDNSMarkerPrefix + instanceID
	request := dnsCaptureRequest{
		Marker:       marker,
		ProxyServers: []string{"127.0.0.1", "::1"},
		ProbeName:    probeName,
	}
	if err := runDNSPowerShell(dnsApplyScript, request); err != nil {
		return nil, fmt.Errorf("apply and verify Windows DNS capture: %w", err)
	}
	return func() error { return CleanupDNSCapture(instanceID) }, nil
}

// CleanupDNSCapture removes only the NRPT rule owned by this client instance.
func CleanupDNSCapture(instanceID string) error {
	request := dnsCaptureRequest{Marker: systemDNSMarkerPrefix + instanceID}
	if err := runDNSPowerShell(dnsCleanupScript, request); err != nil {
		return fmt.Errorf("restore Windows DNS resolution policy: %w", err)
	}
	return nil
}

// DNSCaptureProbeName returns a fresh name answered locally by the proxy. A
// unique label avoids a cached system result during takeover verification.
func DNSCaptureProbeName() (string, error) {
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("create DNS takeover probe name: %w", err)
	}
	return fmt.Sprintf("check-%x.vtrust-dns-check.invalid.", nonce[:]), nil
}

func runDNSPowerShell(script string, request dnsCaptureRequest) error {
	input, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode DNS capture request: %w", err)
	}
	encodedScript := base64.StdEncoding.EncodeToString(utf16le(script))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodedScript)
	command.Stdin = bytes.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("Windows DNS policy command timed out: %s", message)
		}
		return fmt.Errorf("PowerShell DNS policy command failed: %s", message)
	}
	return nil
}

func utf16le(value string) []byte {
	words := utf16.Encode([]rune(value))
	result := make([]byte, len(words)*2)
	for i, word := range words {
		binary.LittleEndian.PutUint16(result[i*2:], word)
	}
	return result
}

const dnsCleanupScript = `$ErrorActionPreference = 'Stop'
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
$rules = @(Get-DnsClientNrptRule -ErrorAction SilentlyContinue | Where-Object { $_.Comment -eq $request.Marker })
foreach ($rule in $rules) {
    Remove-DnsClientNrptRule -Name $rule.Name -Force -Confirm:$false -ErrorAction Stop | Out-Null
}
Clear-DnsClientCache -ErrorAction Stop
`

const dnsApplyScript = `$ErrorActionPreference = 'Stop'
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
$marker = [string]$request.Marker
function Remove-VeloTrustDNSRule {
    $rules = @(Get-DnsClientNrptRule -ErrorAction SilentlyContinue | Where-Object { $_.Comment -eq $marker })
    foreach ($rule in $rules) {
        Remove-DnsClientNrptRule -Name $rule.Name -Force -Confirm:$false -ErrorAction Stop | Out-Null
    }
}
Remove-VeloTrustDNSRule
try {
    Add-DnsClientNrptRule -Namespace '.' -NameServers ([string[]]$request.ProxyServers) -NameEncoding Punycode -Comment $marker -DisplayName 'VeloTrust local DNS proxy' -PassThru -ErrorAction Stop | Out-Null
    Clear-DnsClientCache -ErrorAction Stop
    $effective = @(Get-DnsClientNrptPolicy -Effective -Namespace '.' -ErrorAction Stop)
    $effectiveServers = @($effective | ForEach-Object { $_.NameServers })
    if (-not (($effectiveServers -contains '127.0.0.1') -or ($effectiveServers -contains '::1'))) {
        throw 'effective Windows DNS policy does not point at the VeloTrust local proxy'
    }
    $answer = @(Resolve-DnsName -Name ([string]$request.ProbeName) -Type A -DnsOnly -NoHostsFile -ErrorAction Stop)
    if (-not ($answer | Where-Object { $_.IPAddress -eq '127.0.0.1' })) {
        throw 'system DNS probe did not reach the VeloTrust local proxy'
    }
} catch {
    $failure = $_
    try {
        Remove-VeloTrustDNSRule
        Clear-DnsClientCache -ErrorAction Stop
    } catch {}
    [Console]::Error.WriteLine($failure.Exception.Message)
    exit 1
}
`

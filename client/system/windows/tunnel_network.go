//go:build windows

// SPDX-License-Identifier: BSD-3-Clause

package windows

import (
	"errors"
	"fmt"
	"net/netip"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
)

const (
	maxLifetime        = ^uint32(0)
	forwardProtocol    = 3 // MIB_IPPROTO_NETMGMT
	forwardOrigin      = 0 // NlroManual
	forwardRouteMetric = 25
)

type tunnelNetworkState struct {
	interfaceLUID  uint64
	interfaceIndex uint32
	address        netip.Prefix
	routes         []netip.Prefix
	managedRoutes  []netip.Prefix
}

type ipAddressPrefix struct {
	prefix       windows.RawSockaddrInet6
	prefixLength uint8
	padding      [3]byte
}

type ipForwardRow2 struct {
	interfaceLUID        uint64
	interfaceIndex       uint32
	destinationPrefix    ipAddressPrefix
	nextHop              windows.RawSockaddrInet6
	sitePrefixLength     uint8
	padding              [3]byte
	validLifetime        uint32
	preferredLifetime    uint32
	metric               uint32
	protocol             uint32
	loopback             uint8
	autoconfigureAddress uint8
	publish              uint8
	immortal             uint8
	age                  uint32
	origin               uint32
}

var (
	iphlpapi               = windows.NewLazySystemDLL("iphlpapi.dll")
	createUnicastAddress   = iphlpapi.NewProc("CreateUnicastIpAddressEntry")
	deleteUnicastAddress   = iphlpapi.NewProc("DeleteUnicastIpAddressEntry")
	createForwardEntry     = iphlpapi.NewProc("CreateIpForwardEntry2")
	deleteForwardEntry     = iphlpapi.NewProc("DeleteIpForwardEntry2")
	getUnicastAddressTable = iphlpapi.NewProc("GetUnicastIpAddressTable")
	getForwardTable        = iphlpapi.NewProc("GetIpForwardTable2")
	freeMibTable           = iphlpapi.NewProc("FreeMibTable")
	convertInterfaceLUID   = iphlpapi.NewProc("ConvertInterfaceLuidToIndex")
	initializeAddress      = iphlpapi.NewProc("InitializeUnicastIpAddressEntry")
	initializeRoute        = iphlpapi.NewProc("InitializeIpForwardEntry")
)

type unicastAddressTable struct {
	numEntries uint32
	padding    uint32
	rows       [1]windows.MibUnicastIpAddressRow
}

type forwardRouteTable struct {
	numEntries uint32
	padding    uint32
	rows       [1]ipForwardRow2
}

func ConfigureTunnelNetwork(interfaceName, rawAddress string, rawRoutes []string) (func() error, error) {
	if unsafe.Sizeof(ipForwardRow2{}) != 104 || unsafe.Sizeof(windows.MibUnicastIpAddressRow{}) != 80 || unsafe.Offsetof(forwardRouteTable{}.rows) != 8 || unsafe.Offsetof(unicastAddressTable{}.rows) != 8 {
		return nil, errors.New("Windows networking API structure layout is unsupported on this architecture")
	}
	address, err := netip.ParsePrefix(rawAddress)
	if err != nil {
		return nil, fmt.Errorf("parse tunnel address: %w", err)
	}
	state := &tunnelNetworkState{address: address}
	for _, raw := range rawRoutes {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("parse tunnel route %q: %w", raw, err)
		}
		state.routes = append(state.routes, prefix.Masked())
	}
	adapter, err := wintun.OpenAdapter(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("open Wintun interface: %w", err)
	}
	defer adapter.Close()
	state.interfaceLUID = adapter.LUID()
	if state.interfaceLUID == 0 {
		return nil, errors.New("Wintun interface returned an empty LUID")
	}
	if err := convertInterfaceLUID.Find(); err != nil {
		return nil, fmt.Errorf("load Windows interface index API: %w", err)
	}
	result, _, _ := convertInterfaceLUID.Call(uintptr(unsafe.Pointer(&state.interfaceLUID)), uintptr(unsafe.Pointer(&state.interfaceIndex)))
	if result != 0 {
		return nil, fmt.Errorf("resolve Wintun interface index: %w", syscall.Errno(result))
	}
	runtime.KeepAlive(&state.interfaceLUID)
	runtime.KeepAlive(&state.interfaceIndex)
	if state.interfaceIndex == 0 {
		return nil, errors.New("Wintun interface has no network interface index")
	}

	// Remove routes and manual addresses left by an interrupted prior session.
	// The adapter is reserved for this client and concurrent sessions are locked.
	if err := pruneManagedNetworkState(state.interfaceLUID); err != nil {
		return nil, fmt.Errorf("remove stale tunnel network state: %w", err)
	}
	if err := state.addAddress(); err != nil {
		return nil, err
	}
	for _, route := range state.routes {
		if err := state.addRoute(route); err != nil {
			_ = state.cleanup()
			return nil, err
		}
	}
	return state.cleanup, nil
}

// CleanupTunnelNetwork removes manually managed addresses and routes left on
// the reserved client adapter when a prior process was forcibly terminated.
func CleanupTunnelNetwork(interfaceName string) error {
	adapter, err := wintun.OpenAdapter(interfaceName)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_NOT_FOUND) {
			return nil
		}
		return fmt.Errorf("open Wintun interface for cleanup: %w", err)
	}
	defer adapter.Close()
	interfaceLUID := adapter.LUID()
	if interfaceLUID == 0 {
		return errors.New("Wintun interface returned an empty LUID")
	}
	return pruneManagedNetworkState(interfaceLUID)
}

func (state *tunnelNetworkState) cleanup() error {
	var firstErr error
	for _, route := range state.managedRoutes {
		if err := state.deleteRoute(route); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := state.deleteAddress(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (state *tunnelNetworkState) addAddress() error {
	row := windows.MibUnicastIpAddressRow{}
	if err := callIPHelperInitializer(initializeAddress, unsafe.Pointer(&row)); err != nil {
		return fmt.Errorf("initialize tunnel address: %w", err)
	}
	row.InterfaceLuid = state.interfaceLUID
	row.PrefixOrigin = 1 // IpPrefixOriginManual
	row.SuffixOrigin = 1 // IpSuffixOriginManual
	row.ValidLifetime = maxLifetime
	row.PreferredLifetime = maxLifetime
	row.OnLinkPrefixLength = uint8(state.address.Bits())
	setSockaddr(&row.Address, state.address.Addr())
	if err := callIPHelper(createUnicastAddress, unsafe.Pointer(&row)); err != nil {
		return fmt.Errorf("configure tunnel address: %w", err)
	}
	return nil
}

func (state *tunnelNetworkState) deleteAddress() error {
	row := windows.MibUnicastIpAddressRow{}
	if err := callIPHelperInitializer(initializeAddress, unsafe.Pointer(&row)); err != nil {
		return fmt.Errorf("initialize tunnel address cleanup: %w", err)
	}
	row.InterfaceLuid = state.interfaceLUID
	row.PrefixOrigin = 1
	row.SuffixOrigin = 1
	row.OnLinkPrefixLength = uint8(state.address.Bits())
	setSockaddr(&row.Address, state.address.Addr())
	err := callIPHelper(deleteUnicastAddress, unsafe.Pointer(&row))
	if errors.Is(err, syscall.Errno(windows.ERROR_NOT_FOUND)) {
		return nil
	}
	return err
}

func (state *tunnelNetworkState) addRoute(prefix netip.Prefix) error {
	row, err := state.routeRow(prefix)
	if err != nil {
		return fmt.Errorf("initialize tunnel route %s: %w", prefix, err)
	}
	if err := callIPHelper(createForwardEntry, unsafe.Pointer(&row)); err != nil {
		return fmt.Errorf("configure tunnel route %s: %w", prefix, err)
	}
	state.managedRoutes = append(state.managedRoutes, prefix)
	return nil
}

func (state *tunnelNetworkState) deleteRoute(prefix netip.Prefix) error {
	row, err := state.routeRow(prefix)
	if err != nil {
		return fmt.Errorf("initialize tunnel route cleanup %s: %w", prefix, err)
	}
	err = callIPHelper(deleteForwardEntry, unsafe.Pointer(&row))
	if errors.Is(err, syscall.Errno(windows.ERROR_NOT_FOUND)) {
		return nil
	}
	return err
}

func (state *tunnelNetworkState) routeRow(prefix netip.Prefix) (ipForwardRow2, error) {
	row := ipForwardRow2{}
	if err := callIPHelperInitializer(initializeRoute, unsafe.Pointer(&row)); err != nil {
		return ipForwardRow2{}, err
	}
	row.interfaceLUID = state.interfaceLUID
	row.interfaceIndex = state.interfaceIndex
	row.metric = forwardRouteMetric
	row.protocol = forwardProtocol
	row.loopback = 0
	row.autoconfigureAddress = 0
	row.publish = 0
	row.immortal = 0
	row.origin = forwardOrigin
	masked := prefix.Masked()
	setSockaddr(&row.destinationPrefix.prefix, masked.Addr())
	row.destinationPrefix.prefixLength = uint8(masked.Bits())
	setSockaddr(&row.nextHop, netip.IPv4Unspecified())
	if masked.Addr().Is6() {
		setSockaddr(&row.nextHop, netip.IPv6Unspecified())
	}
	return row, nil
}

func setSockaddr(destination *windows.RawSockaddrInet6, address netip.Addr) {
	*destination = windows.RawSockaddrInet6{}
	if address.Is4() {
		addr4 := (*windows.RawSockaddrInet4)(unsafe.Pointer(destination))
		addr4.Family = windows.AF_INET
		ipv4 := address.As4()
		copy(addr4.Addr[:], ipv4[:])
		return
	}
	destination.Family = windows.AF_INET6
	ipv6 := address.As16()
	copy(destination.Addr[:], ipv6[:])
}

func callIPHelper(proc *windows.LazyProc, row unsafe.Pointer) error {
	if err := proc.Find(); err != nil {
		return err
	}
	result, _, _ := proc.Call(uintptr(row))
	runtime.KeepAlive(row)
	if result != 0 {
		return syscall.Errno(result)
	}
	return nil
}

func callIPHelperInitializer(proc *windows.LazyProc, row unsafe.Pointer) error {
	if err := proc.Find(); err != nil {
		return err
	}
	proc.Call(uintptr(row))
	runtime.KeepAlive(row)
	return nil
}

func pruneManagedNetworkState(interfaceLUID uint64) error {
	var cleanupErrors []error
	forwardTable, err := getIPHelperTable(getForwardTable)
	if err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("list tunnel routes: %w", err))
	} else {
		table := (*forwardRouteTable)(forwardTable)
		rowSize := unsafe.Sizeof(ipForwardRow2{})
		rows := unsafe.Add(forwardTable, unsafe.Offsetof(forwardRouteTable{}.rows))
		for index := uint32(0); index < table.numEntries; index++ {
			row := *(*ipForwardRow2)(unsafe.Add(rows, uintptr(index)*rowSize))
			if row.interfaceLUID != interfaceLUID || row.protocol != forwardProtocol {
				continue
			}
			if err := callIPHelper(deleteForwardEntry, unsafe.Pointer(&row)); err != nil && !errors.Is(err, syscall.Errno(windows.ERROR_NOT_FOUND)) {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("remove stale tunnel route: %w", err))
			}
		}
		freeIPHelperTable(forwardTable)
	}

	addressTable, err := getIPHelperTable(getUnicastAddressTable)
	if err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("list tunnel addresses: %w", err))
	} else {
		table := (*unicastAddressTable)(addressTable)
		rowSize := unsafe.Sizeof(windows.MibUnicastIpAddressRow{})
		rows := unsafe.Add(addressTable, unsafe.Offsetof(unicastAddressTable{}.rows))
		for index := uint32(0); index < table.numEntries; index++ {
			row := *(*windows.MibUnicastIpAddressRow)(unsafe.Add(rows, uintptr(index)*rowSize))
			if row.InterfaceLuid != interfaceLUID || row.PrefixOrigin != 1 || row.SuffixOrigin != 1 {
				continue
			}
			if err := callIPHelper(deleteUnicastAddress, unsafe.Pointer(&row)); err != nil && !errors.Is(err, syscall.Errno(windows.ERROR_NOT_FOUND)) {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("remove stale tunnel address: %w", err))
			}
		}
		freeIPHelperTable(addressTable)
	}
	return errors.Join(cleanupErrors...)
}

func getIPHelperTable(proc *windows.LazyProc) (unsafe.Pointer, error) {
	if err := proc.Find(); err != nil {
		return nil, err
	}
	var table unsafe.Pointer
	result, _, _ := proc.Call(uintptr(windows.AF_UNSPEC), uintptr(unsafe.Pointer(&table)))
	runtime.KeepAlive(&table)
	if result != 0 {
		return nil, syscall.Errno(result)
	}
	if table == nil {
		return nil, errors.New("Windows returned an empty network table")
	}
	return table, nil
}

func freeIPHelperTable(table unsafe.Pointer) {
	if freeMibTable.Find() == nil {
		freeMibTable.Call(uintptr(table))
		runtime.KeepAlive(table)
	}
}

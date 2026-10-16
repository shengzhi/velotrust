//go:build linux && !android

// SPDX-License-Identifier: BSD-3-Clause

package sharedsock

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

var (
	ErrProbeClosed      = errors.New("source probe is closed")
	ErrProbeUnsupported = errors.New("source probe is supported only on Linux")
)

// RouteError indicates that the kernel could not find a usable route for a
// destination. The probe socket remains usable after this error.
type RouteError struct {
	Err error
}

func (e *RouteError) Error() string { return fmt.Sprintf("route lookup: %v", e.Err) }
func (e *RouteError) Unwrap() error { return e.Err }

// SourceProbe asks the kernel which local address it would use to reach a
// destination, without sending a packet. An optional mark makes the lookup
// follow the same policy-routing rules as a marked data socket.
type SourceProbe struct {
	mu     sync.Mutex
	family int
	mark   uint32
	fd     int
	closed bool
}

// NewSourceProbe creates a reusable IPv4 or IPv6 route probe.
func NewSourceProbe(family int, mark uint32) (*SourceProbe, error) {
	if family != syscall.AF_INET && family != syscall.AF_INET6 {
		return nil, fmt.Errorf("unsupported address family %d", family)
	}
	p := &SourceProbe{family: family, mark: mark, fd: -1}
	if err := p.openLocked(); err != nil {
		return nil, err
	}
	return p, nil
}

// Resolve returns the source address selected by the kernel for dst.
func (p *SourceProbe) Resolve(dst netip.Addr) (netip.Addr, error) {
	if !dst.IsValid() {
		return netip.Addr{}, errors.New("invalid destination address")
	}
	dst = dst.Unmap()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return netip.Addr{}, ErrProbeClosed
	}
	if (p.family == syscall.AF_INET) != dst.Is4() {
		return netip.Addr{}, fmt.Errorf("destination %s does not match probe family", dst)
	}
	if p.fd < 0 {
		if err := p.openLocked(); err != nil {
			return netip.Addr{}, err
		}
	}

	src, err := p.lookupLocked(dst)
	if err != nil {
		var routeErr *RouteError
		if !errors.As(err, &routeErr) {
			_ = p.closeFDLocked()
		}
		return netip.Addr{}, err
	}
	return src, nil
}

// Close releases the probe socket. Repeated calls are safe.
func (p *SourceProbe) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return p.closeFDLocked()
}

func (p *SourceProbe) openLocked() error {
	fd, err := unix.Socket(p.family, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, unix.IPPROTO_UDP)
	if err != nil {
		return fmt.Errorf("create source probe socket: %w", err)
	}
	if p.mark != 0 {
		if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_MARK, int(p.mark)); err != nil {
			_ = unix.Close(fd)
			return fmt.Errorf("set source probe socket mark: %w", err)
		}
	}
	p.fd = fd
	return nil
}

func (p *SourceProbe) lookupLocked(dst netip.Addr) (netip.Addr, error) {
	if err := disconnectProbe(p.fd); err != nil {
		return netip.Addr{}, fmt.Errorf("disconnect source probe: %w", err)
	}
	sa, err := probeSockaddr(dst)
	if err != nil {
		return netip.Addr{}, err
	}
	if err := unix.Connect(p.fd, sa); err != nil {
		return netip.Addr{}, &RouteError{Err: err}
	}
	local, err := unix.Getsockname(p.fd)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("read source probe address: %w", err)
	}

	var src netip.Addr
	switch addr := local.(type) {
	case *unix.SockaddrInet4:
		src = netip.AddrFrom4(addr.Addr)
	case *unix.SockaddrInet6:
		src = netip.AddrFrom16(addr.Addr)
	}
	if !src.IsValid() || src.IsUnspecified() {
		return netip.Addr{}, &RouteError{Err: errors.New("no source address")}
	}
	return src.Unmap(), nil
}

func (p *SourceProbe) closeFDLocked() error {
	if p.fd < 0 {
		return nil
	}
	fd := p.fd
	p.fd = -1
	if err := unix.Close(fd); err != nil && !errors.Is(err, unix.EBADF) {
		return err
	}
	return nil
}

func probeSockaddr(dst netip.Addr) (unix.Sockaddr, error) {
	if dst.Is4() {
		return &unix.SockaddrInet4{Addr: dst.As4()}, nil
	}
	var zone uint32
	if name := dst.Zone(); name != "" {
		index, err := strconv.ParseUint(name, 10, 32)
		if err != nil {
			iface, lookupErr := net.InterfaceByName(name)
			if lookupErr != nil {
				return nil, fmt.Errorf("resolve IPv6 zone %q: %w", name, lookupErr)
			}
			zone = uint32(iface.Index)
		} else {
			zone = uint32(index)
		}
	}
	return &unix.SockaddrInet6{Addr: dst.As16(), ZoneId: zone}, nil
}

func disconnectProbe(fd int) error {
	addr := unix.RawSockaddr{Family: unix.AF_UNSPEC}
	_, _, errno := syscall.Syscall(syscall.SYS_CONNECT, uintptr(fd), uintptr(unsafe.Pointer(&addr)), unsafe.Sizeof(addr))
	if errno != 0 {
		return errno
	}
	return nil
}

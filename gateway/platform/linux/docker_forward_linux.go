//go:build linux

package linux

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os/exec"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"velotrust/gateway/config"
)

const (
	iptablesFilterTable = "filter"
	dockerForwardChain  = "DOCKER-FORWARD"
	dockerLegacyChain   = "DOCKER"
	dockerUserChain     = "DOCKER-USER"
)

type iptablesFamily struct {
	protocol iptables.Protocol
	name     string
}

var supportedIPTablesFamilies = []iptablesFamily{
	{protocol: iptables.ProtocolIPv4, name: "IPv4"},
	{protocol: iptables.ProtocolIPv6, name: "IPv6"},
}

// RefreshDockerFirewall reapplies the optional Docker hook after Docker may
// have recreated its iptables chains. It does not change VeloTrust's ACL.
func (d *Driver) RefreshDockerFirewall(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.replaceDockerForwardRulesLocked(); err != nil {
		return fmt.Errorf("refresh Docker forwarding integration: %w", err)
	}
	return nil
}

// replaceDockerForwardRulesLocked mirrors VeloTrust allow rules into Docker's
// DOCKER-USER hook when Docker's iptables forwarding chains are present. The
// VeloTrust nftables chain remains the policy authority and runs before Docker.
func (d *Driver) replaceDockerForwardRulesLocked() error {
	var result error
	for _, family := range supportedIPTablesFamilies {
		ipt, err := newIPTables(family.protocol)
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				slogDebugDockerIPTablesUnavailable(family.name)
				continue
			}
			result = errors.Join(result, fmt.Errorf("initialize %s iptables: %w", family.name, err))
			continue
		}

		present, err := dockerForwardingPresent(ipt)
		if err != nil {
			result = errors.Join(result, fmt.Errorf("inspect Docker %s forwarding chains: %w", family.name, err))
			continue
		}
		if !present {
			slog.Debug("Docker forwarding chains not detected; Docker integration is inactive", "family", family.name, "interface", d.interfaceName)
			if err := removeDockerManagedChain(ipt, d.interfaceName); err != nil {
				result = errors.Join(result, fmt.Errorf("remove stale Docker %s forwarding rules: %w", family.name, err))
			}
			continue
		}

		var desired [][]string
		if !d.blocked {
			for _, rule := range d.rules {
				if rule.Action != "allow" || !ruleMatchesFamily(rule, family.protocol) {
					continue
				}
				forward, reply, err := dockerACLRuleSpecs(d.interfaceName, rule)
				if err != nil {
					result = errors.Join(result, fmt.Errorf("build Docker %s rule %q: %w", family.name, rule.ID, err))
					continue
				}
				desired = append(desired, forward, reply)
			}
		}
		if len(desired) == 0 {
			if err := removeDockerManagedChain(ipt, d.interfaceName); err != nil {
				result = errors.Join(result, fmt.Errorf("remove Docker %s forwarding rules: %w", family.name, err))
			}
			continue
		}
		if err := replaceDockerManagedChain(ipt, d.interfaceName, desired); err != nil {
			result = errors.Join(result, fmt.Errorf("apply Docker %s forwarding rules: %w", family.name, err))
			continue
		}
		slog.Debug("synchronized VeloTrust rules with Docker DOCKER-USER", "family", family.name, "interface", d.interfaceName, "chain", dockerManagedChainName(d.interfaceName), "rules", len(desired))
	}
	return result
}

func (d *Driver) removeDockerForwardRulesLocked() error {
	var result error
	for _, family := range supportedIPTablesFamilies {
		ipt, err := newIPTables(family.protocol)
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				continue
			}
			result = errors.Join(result, fmt.Errorf("initialize %s iptables for cleanup: %w", family.name, err))
			continue
		}
		if err := removeDockerManagedChain(ipt, d.interfaceName); err != nil {
			result = errors.Join(result, fmt.Errorf("remove Docker %s forwarding rules: %w", family.name, err))
		}
	}
	return result
}

func newIPTables(protocol iptables.Protocol) (*iptables.IPTables, error) {
	return iptables.New(iptables.IPFamily(protocol), iptables.Timeout(5))
}

func dockerForwardingPresent(ipt *iptables.IPTables) (bool, error) {
	forwarderExists, err := ipt.ChainExists(iptablesFilterTable, dockerForwardChain)
	if err != nil {
		return false, err
	}
	if !forwarderExists {
		forwarderExists, err = ipt.ChainExists(iptablesFilterTable, dockerLegacyChain)
		if err != nil {
			return false, err
		}
	}
	if !forwarderExists {
		return false, nil
	}

	forwardExists, err := ipt.ChainExists(iptablesFilterTable, "FORWARD")
	if err != nil {
		return false, err
	}
	if !forwardExists {
		return false, errors.New("Docker forwarding chain exists but filter/FORWARD is missing")
	}
	userExists, err := ipt.ChainExists(iptablesFilterTable, dockerUserChain)
	if err != nil {
		return false, err
	}
	if !userExists {
		return false, errors.New("Docker forwarding chain exists but filter/DOCKER-USER is missing")
	}
	linked, err := ipt.Exists(iptablesFilterTable, "FORWARD", "-j", dockerUserChain)
	if err != nil {
		return false, err
	}
	if !linked {
		return false, errors.New("filter/DOCKER-USER is not linked from filter/FORWARD")
	}
	return true, nil
}

func replaceDockerManagedChain(ipt *iptables.IPTables, interfaceName string, rules [][]string) error {
	chainName := dockerManagedChainName(interfaceName)
	exists, err := ipt.ChainExists(iptablesFilterTable, chainName)
	if err != nil {
		return err
	}
	if !exists {
		if err := ipt.NewChain(iptablesFilterTable, chainName); err != nil {
			return fmt.Errorf("create chain %s: %w", chainName, err)
		}
	}
	if err := ipt.ClearChain(iptablesFilterTable, chainName); err != nil {
		return fmt.Errorf("clear chain %s: %w", chainName, err)
	}
	for _, rule := range rules {
		if err := ipt.Append(iptablesFilterTable, chainName, rule...); err != nil {
			return fmt.Errorf("append rule to chain %s: %w", chainName, err)
		}
	}
	return installDockerUserHook(ipt, interfaceName, chainName)
}

func removeDockerManagedChain(ipt *iptables.IPTables, interfaceName string) error {
	chainName := dockerManagedChainName(interfaceName)
	if err := removeDockerUserHook(ipt, interfaceName, chainName); err != nil {
		return err
	}
	exists, err := ipt.ChainExists(iptablesFilterTable, chainName)
	if err != nil || !exists {
		return err
	}
	if err := ipt.ClearAndDeleteChain(iptablesFilterTable, chainName); err != nil {
		return fmt.Errorf("delete chain %s: %w", chainName, err)
	}
	return nil
}

func installDockerUserHook(ipt *iptables.IPTables, interfaceName, chainName string) error {
	if err := removeDockerUserHook(ipt, interfaceName, chainName); err != nil {
		return err
	}
	position, err := dockerUserInsertPosition(ipt)
	if err != nil {
		return err
	}
	if err := ipt.InsertUnique(iptablesFilterTable, dockerUserChain, position, dockerUserHookSpec(interfaceName, chainName)...); err != nil {
		return fmt.Errorf("insert managed jump into DOCKER-USER: %w", err)
	}
	return nil
}

func removeDockerUserHook(ipt *iptables.IPTables, interfaceName, chainName string) error {
	// iptables -C returns status 2 when a jump target chain is absent. Check
	// our owned chain first so first startup and repeated cleanup are no-ops.
	managedChainExists, err := ipt.ChainExists(iptablesFilterTable, chainName)
	if err != nil || !managedChainExists {
		return err
	}
	userExists, err := ipt.ChainExists(iptablesFilterTable, dockerUserChain)
	if err != nil || !userExists {
		return err
	}
	spec := dockerUserHookSpec(interfaceName, chainName)
	for {
		exists, err := ipt.Exists(iptablesFilterTable, dockerUserChain, spec...)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		if err := ipt.Delete(iptablesFilterTable, dockerUserChain, spec...); err != nil {
			return fmt.Errorf("delete managed DOCKER-USER jump: %w", err)
		}
	}
}

func dockerUserInsertPosition(ipt *iptables.IPTables) (int, error) {
	rules, err := ipt.List(iptablesFilterTable, dockerUserChain)
	if err != nil {
		return 0, fmt.Errorf("list DOCKER-USER rules: %w", err)
	}
	position := 1
	for _, rule := range rules {
		fields := strings.Fields(rule)
		if len(fields) < 3 || fields[0] != "-A" || fields[1] != dockerUserChain {
			continue
		}
		for index := 2; index+1 < len(fields); index++ {
			if (fields[index] == "-j" || fields[index] == "--jump") && fields[index+1] == "RETURN" {
				return position, nil
			}
		}
		position++
	}
	return position, nil
}

func dockerUserHookSpec(interfaceName, chainName string) []string {
	return []string{"-m", "comment", "--comment", "velotrust-gateway:" + interfaceName, "-j", chainName}
}

func dockerManagedChainName(interfaceName string) string {
	digest := sha256.Sum256([]byte(interfaceName))
	return "VTGW-" + strings.ToUpper(hex.EncodeToString(digest[:8]))
}

func ruleMatchesFamily(rule config.ACLRule, protocol iptables.Protocol) bool {
	source, err := netip.ParsePrefix(rule.SourcePrefix)
	if err != nil {
		return false
	}
	return source.Addr().Is4() == (protocol == iptables.ProtocolIPv4)
}

func dockerACLRuleSpecs(interfaceName string, rule config.ACLRule) ([]string, []string, error) {
	if _, err := netip.ParsePrefix(rule.SourcePrefix); err != nil {
		return nil, nil, fmt.Errorf("parse source prefix %q: %w", rule.SourcePrefix, err)
	}
	if _, err := netip.ParsePrefix(rule.Destination); err != nil {
		return nil, nil, fmt.Errorf("parse destination prefix %q: %w", rule.Destination, err)
	}

	forward := []string{"-i", interfaceName, "-s", rule.SourcePrefix, "-d", rule.Destination}
	reply := []string{"-o", interfaceName, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-s", rule.Destination, "-d", rule.SourcePrefix}
	protocol := rule.Protocol
	if protocol == "icmpv6" {
		protocol = "ipv6-icmp"
	}
	if protocol != "any" {
		forward = append(forward, "-p", protocol)
		reply = append(reply, "-p", protocol)
	}
	if rule.PortFrom != 0 {
		portRange := formatIPTablesPortRange(rule.PortFrom, rule.PortTo)
		forward = append(forward, "--dport", portRange)
		reply = append(reply, "--sport", portRange)
	}
	forward = append(forward, "-j", "ACCEPT")
	reply = append(reply, "-j", "ACCEPT")
	return forward, reply, nil
}

func formatIPTablesPortRange(from, to uint16) string {
	if from == to {
		return fmt.Sprint(from)
	}
	return fmt.Sprintf("%d:%d", from, to)
}

func slogDebugDockerIPTablesUnavailable(family string) {
	slog.Debug("iptables executable is unavailable; Docker forwarding integration is skipped", "family", family)
}

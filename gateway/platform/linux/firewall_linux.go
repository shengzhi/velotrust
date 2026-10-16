//go:build linux

package linux

import (
	"fmt"
	"net/netip"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
	"velotrust/gateway/config"
)

func (d *Driver) replaceFirewallLocked() error {
	conn, err := nftables.New()
	if err != nil {
		return fmt.Errorf("open nftables connection: %w", err)
	}
	table, chain, exists, err := ensureChain(conn, filterTableName, forwardChainName, nftables.TableFamilyINet, nftables.ChainHookForward, -10, nftables.ChainTypeFilter)
	if err != nil {
		return err
	}
	if exists {
		conn.FlushChain(chain)
	}
	chain.Table = table

	if d.blocked {
		conn.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: append(ifnameMatch(expr.MetaKeyIIFNAME, d.interfaceName), &expr.Verdict{Kind: expr.VerdictDrop})})
		conn.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: append(ifnameMatch(expr.MetaKeyOIFNAME, d.interfaceName), &expr.Verdict{Kind: expr.VerdictDrop})})
	} else {
		for _, rule := range d.rules {
			expressions, err := firewallRuleExpressions(d.interfaceName, rule)
			if err != nil {
				return err
			}
			conn.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: expressions})
			replyExpressions, err := firewallReplyExpressions(d.interfaceName, rule)
			if err != nil {
				return err
			}
			conn.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: replyExpressions})
		}
		conn.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: append(ifnameMatch(expr.MetaKeyIIFNAME, d.interfaceName), &expr.Verdict{Kind: expr.VerdictDrop})})
		conn.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: append(ifnameMatch(expr.MetaKeyOIFNAME, d.interfaceName), &expr.Verdict{Kind: expr.VerdictDrop})})
	}
	if err := conn.Flush(); err != nil {
		return fmt.Errorf("commit nftables forwarding rules: %w", err)
	}
	if err := d.replaceDockerForwardRulesLocked(); err != nil {
		return fmt.Errorf("synchronize Docker forwarding rules: %w", err)
	}
	return nil
}

func (d *Driver) replaceNATLocked(mappings []config.NATMapping) error {
	conn, err := nftables.New()
	if err != nil {
		return fmt.Errorf("open nftables connection: %w", err)
	}
	table, prerouting, preroutingExists, err := ensureChain(conn, natTableName, preroutingChainName, nftables.TableFamilyINet, nftables.ChainHookPrerouting, -101, nftables.ChainTypeNAT)
	if err != nil {
		return err
	}
	postrouting, postroutingExists, err := ensureChainInTable(conn, table, postroutingChainName, nftables.ChainHookPostrouting, *nftables.ChainPriorityNATSource, nftables.ChainTypeNAT)
	if err != nil {
		return err
	}
	if preroutingExists {
		conn.FlushChain(prerouting)
	}
	if postroutingExists {
		conn.FlushChain(postrouting)
	}
	for _, mapping := range mappings {
		var chain *nftables.Chain
		var expressions []expr.Any
		if mapping.Kind() == config.NATTypeSNAT {
			chain = postrouting
			expressions, err = snatRuleExpressions(d.interfaceName, mapping)
		} else {
			chain = prerouting
			expressions, err = natRuleExpressions(d.interfaceName, mapping)
		}
		if err != nil {
			return err
		}
		conn.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: expressions})
	}
	if err := conn.Flush(); err != nil {
		return fmt.Errorf("commit nftables NAT rules: %w", err)
	}
	return nil
}

func ensureChainInTable(conn *nftables.Conn, table *nftables.Table, chainName string, hook *nftables.ChainHook, priority nftables.ChainPriority, chainType nftables.ChainType) (*nftables.Chain, bool, error) {
	chains, err := conn.ListChains()
	if err != nil {
		return nil, false, fmt.Errorf("list nftables chains: %w", err)
	}
	for _, candidate := range chains {
		if candidate.Table != nil && candidate.Table.Name == table.Name && candidate.Table.Family == table.Family && candidate.Name == chainName {
			return &nftables.Chain{
				Name: chainName, Table: table, Hooknum: hook,
				Priority: nftables.ChainPriorityRef(priority), Type: chainType,
			}, true, nil
		}
	}
	chain := conn.AddChain(&nftables.Chain{
		Name: chainName, Table: table, Hooknum: hook,
		Priority: nftables.ChainPriorityRef(priority), Type: chainType,
	})
	return chain, false, nil
}

func ensureChain(conn *nftables.Conn, tableName, chainName string, family nftables.TableFamily, hook *nftables.ChainHook, priority nftables.ChainPriority, chainType nftables.ChainType) (*nftables.Table, *nftables.Chain, bool, error) {
	tables, err := conn.ListTables()
	if err != nil {
		return nil, nil, false, fmt.Errorf("list nftables tables: %w", err)
	}
	var table *nftables.Table
	for _, candidate := range tables {
		if candidate.Name == tableName && candidate.Family == family {
			table = candidate
			break
		}
	}
	newTable := table == nil
	var policy *nftables.ChainPolicy
	if chainType == nftables.ChainTypeFilter {
		policy = policyAccept()
	}
	if newTable {
		table = conn.AddTable(&nftables.Table{Name: tableName, Family: family})
		chain := conn.AddChain(&nftables.Chain{
			Name: chainName, Table: table, Hooknum: hook,
			Priority: nftables.ChainPriorityRef(priority), Type: chainType,
			Policy: policy,
		})
		return table, chain, false, nil
	}
	chains, err := conn.ListChains()
	if err != nil {
		return nil, nil, false, fmt.Errorf("list nftables chains: %w", err)
	}
	for _, candidate := range chains {
		if candidate.Table != nil && candidate.Table.Name == tableName && candidate.Table.Family == family && candidate.Name == chainName {
			return table, &nftables.Chain{
				Name: chainName, Table: table, Hooknum: hook,
				Priority: nftables.ChainPriorityRef(priority), Type: chainType,
				Policy: policy,
			}, true, nil
		}
	}
	chain := conn.AddChain(&nftables.Chain{
		Name: chainName, Table: table, Hooknum: hook,
		Priority: nftables.ChainPriorityRef(priority), Type: chainType,
		Policy: policy,
	})
	return table, chain, false, nil
}

func policyAccept() *nftables.ChainPolicy {
	policy := nftables.ChainPolicyAccept
	return &policy
}

func firewallRuleExpressions(interfaceName string, rule config.ACLRule) ([]expr.Any, error) {
	return firewallExpressionsForDirection(interfaceName, rule, false)
}

func firewallReplyExpressions(interfaceName string, rule config.ACLRule) ([]expr.Any, error) {
	return firewallExpressionsForDirection(interfaceName, rule, true)
}

func firewallExpressionsForDirection(interfaceName string, rule config.ACLRule, reply bool) ([]expr.Any, error) {
	source, err := netip.ParsePrefix(rule.SourcePrefix)
	if err != nil {
		return nil, fmt.Errorf("parse ACL source prefix %q: %w", rule.SourcePrefix, err)
	}
	destination, err := netip.ParsePrefix(rule.Destination)
	if err != nil {
		return nil, fmt.Errorf("parse ACL destination prefix %q: %w", rule.Destination, err)
	}
	ingress := expr.MetaKeyIIFNAME
	if reply {
		ingress = expr.MetaKeyOIFNAME
	}
	expressions := append([]expr.Any(nil), ifnameMatch(ingress, interfaceName)...)
	if reply {
		expressions = append(expressions, establishedMatch()...)
		expressions = append(expressions, familyMatch(destination.Addr())...)
		expressions = append(expressions, prefixMatch(destination.Masked(), true)...)
		expressions = append(expressions, prefixMatch(source.Masked(), false)...)
	} else {
		expressions = append(expressions, familyMatch(source.Addr())...)
		expressions = append(expressions, prefixMatch(source.Masked(), true)...)
		expressions = append(expressions, prefixMatch(destination.Masked(), false)...)
	}
	protocol := rule.Protocol
	switch protocol {
	case "any":
	case "tcp":
		expressions = append(expressions, protocolMatch(unix.IPPROTO_TCP)...)
	case "udp":
		expressions = append(expressions, protocolMatch(unix.IPPROTO_UDP)...)
	case "icmp":
		expressions = append(expressions, protocolMatch(unix.IPPROTO_ICMP)...)
	case "icmpv6":
		expressions = append(expressions, protocolMatch(unix.IPPROTO_ICMPV6)...)
	default:
		return nil, fmt.Errorf("unsupported ACL protocol %q", protocol)
	}
	if rule.PortFrom != 0 {
		portOffset := uint32(2)
		if reply {
			portOffset = 0
		}
		expressions = append(expressions,
			&expr.Payload{DestRegister: unix.NFT_REG_1, Base: expr.PayloadBaseTransportHeader, Offset: portOffset, Len: 2},
			&expr.Cmp{Op: expr.CmpOpGte, Register: unix.NFT_REG_1, Data: []byte{byte(rule.PortFrom >> 8), byte(rule.PortFrom)}},
			&expr.Cmp{Op: expr.CmpOpLte, Register: unix.NFT_REG_1, Data: []byte{byte(rule.PortTo >> 8), byte(rule.PortTo)}},
		)
	}
	verdict := expr.VerdictAccept
	if rule.Action == "deny" {
		verdict = expr.VerdictDrop
	}
	return append(expressions, &expr.Verdict{Kind: verdict}), nil
}

func natRuleExpressions(interfaceName string, mapping config.NATMapping) ([]expr.Any, error) {
	shadow, err := netip.ParsePrefix(mapping.ShadowPrefix)
	if err != nil {
		return nil, fmt.Errorf("parse NAT source prefix %q: %w", mapping.ShadowPrefix, err)
	}
	real, err := netip.ParsePrefix(mapping.RealPrefix)
	if err != nil {
		return nil, fmt.Errorf("parse NAT target prefix %q: %w", mapping.RealPrefix, err)
	}
	first := real.Masked().Addr().AsSlice()
	last := prefixLastAddress(real.Masked()).AsSlice()
	family := uint32(unix.NFPROTO_IPV6)
	if shadow.Addr().Is4() {
		family = unix.NFPROTO_IPV4
	}
	expressions := append([]expr.Any(nil), ifnameMatch(expr.MetaKeyIIFNAME, interfaceName)...)
	expressions = append(expressions, familyMatch(shadow.Addr())...)
	expressions = append(expressions, prefixMatch(shadow.Masked(), false)...)
	expressions = append(expressions,
		&expr.Immediate{Register: unix.NFT_REG_1, Data: first},
		&expr.Immediate{Register: unix.NFT_REG_2, Data: last},
		&expr.NAT{Type: expr.NATTypeDestNAT, Family: family, RegAddrMin: unix.NFT_REG_1, RegAddrMax: unix.NFT_REG_2, Prefix: true},
	)
	return expressions, nil
}

func snatRuleExpressions(tunnelInterface string, mapping config.NATMapping) ([]expr.Any, error) {
	source, err := netip.ParsePrefix(mapping.SourcePrefix)
	if err != nil {
		return nil, fmt.Errorf("parse SNAT source prefix %q: %w", mapping.SourcePrefix, err)
	}
	destination, err := netip.ParsePrefix(mapping.DestinationPrefix)
	if err != nil {
		return nil, fmt.Errorf("parse SNAT destination prefix %q: %w", mapping.DestinationPrefix, err)
	}
	toAddress, err := netip.ParseAddr(mapping.ToAddress)
	if err != nil {
		return nil, fmt.Errorf("parse SNAT address %q: %w", mapping.ToAddress, err)
	}
	family := uint32(unix.NFPROTO_IPV6)
	if source.Addr().Is4() {
		family = unix.NFPROTO_IPV4
	}
	expressions := append([]expr.Any(nil), ifnameMatch(expr.MetaKeyIIFNAME, tunnelInterface)...)
	expressions = append(expressions, ifnameMatch(expr.MetaKeyOIFNAME, mapping.EgressInterface)...)
	expressions = append(expressions, familyMatch(source.Addr())...)
	expressions = append(expressions, prefixMatch(source.Masked(), true)...)
	expressions = append(expressions, prefixMatch(destination.Masked(), false)...)
	expressions = append(expressions,
		&expr.Immediate{Register: unix.NFT_REG_1, Data: toAddress.AsSlice()},
		&expr.NAT{Type: expr.NATTypeSourceNAT, Family: family, RegAddrMin: unix.NFT_REG_1},
	)
	return expressions, nil
}

func ifnameMatch(key expr.MetaKey, interfaceName string) []expr.Any {
	data := make([]byte, unix.IFNAMSIZ)
	copy(data, interfaceName)
	return []expr.Any{
		&expr.Meta{Key: key, Register: unix.NFT_REG_1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: unix.NFT_REG_1, Data: data},
	}
}

func familyMatch(address netip.Addr) []expr.Any {
	family := byte(unix.NFPROTO_IPV6)
	if address.Is4() {
		family = unix.NFPROTO_IPV4
	}
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: unix.NFT_REG_1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: unix.NFT_REG_1, Data: []byte{family}},
	}
}

func protocolMatch(protocol byte) []expr.Any {
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: unix.NFT_REG_1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: unix.NFT_REG_1, Data: []byte{protocol}},
	}
}

func prefixMatch(prefix netip.Prefix, source bool) []expr.Any {
	prefix = prefix.Masked()
	address := prefix.Addr()
	var offset uint32
	var mask []byte
	if address.Is4() {
		if source {
			offset = 12
		} else {
			offset = 16
		}
		mask = netMask(prefix.Bits(), 32)
	} else {
		if source {
			offset = 8
		} else {
			offset = 24
		}
		mask = netMask(prefix.Bits(), 128)
	}
	return []expr.Any{
		&expr.Payload{DestRegister: unix.NFT_REG_1, Base: expr.PayloadBaseNetworkHeader, Offset: offset, Len: uint32(len(mask))},
		&expr.Bitwise{SourceRegister: unix.NFT_REG_1, DestRegister: unix.NFT_REG_1, Len: uint32(len(mask)), Mask: mask, Xor: make([]byte, len(mask))},
		&expr.Cmp{Op: expr.CmpOpEq, Register: unix.NFT_REG_1, Data: address.AsSlice()},
	}
}

func establishedMatch() []expr.Any {
	return []expr.Any{
		&expr.Ct{Key: expr.CtKeySTATE, Register: unix.NFT_REG_1},
		&expr.Bitwise{
			SourceRegister: unix.NFT_REG_1, DestRegister: unix.NFT_REG_1, Len: 4,
			Mask: binaryutil.NativeEndian.PutUint32(expr.CtStateBitESTABLISHED | expr.CtStateBitRELATED),
			Xor:  binaryutil.NativeEndian.PutUint32(0),
		},
		&expr.Cmp{Op: expr.CmpOpNeq, Register: unix.NFT_REG_1, Data: []byte{0, 0, 0, 0}},
	}
}

func netMask(bits, total int) []byte {
	mask := make([]byte, total/8)
	for index := range mask {
		remaining := bits - index*8
		switch {
		case remaining >= 8:
			mask[index] = 0xff
		case remaining > 0:
			mask[index] = byte(0xff << (8 - remaining))
		}
	}
	return mask
}

func prefixLastAddress(prefix netip.Prefix) netip.Addr {
	address := prefix.Masked().Addr()
	bytes := address.AsSlice()
	mask := netMask(prefix.Bits(), address.BitLen())
	for index := range bytes {
		bytes[index] |= ^mask[index]
	}
	if address.Is4() {
		var last [4]byte
		copy(last[:], bytes)
		return netip.AddrFrom4(last)
	}
	var last [16]byte
	copy(last[:], bytes)
	return netip.AddrFrom16(last)
}

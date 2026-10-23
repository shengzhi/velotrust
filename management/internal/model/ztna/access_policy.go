package ztna

// AccessPolicyRule is a logical authorization outcome applied to the policy's
// subjects and protected resources. Network addresses and service ports come
// from the resource catalog and active Client leases, not from this rule.
type AccessPolicyRule struct {
	ID       string `json:"id"`
	Priority uint32 `json:"priority"`
	Action   string `json:"action"`
}

// AccessPolicyView is the RPC projection of current policy data. VersionNo is
// an optimistic persistence token only; it is not a policy revision.
type AccessPolicyView struct {
	ID          int64              `json:"id"`
	VersionNo   int64              `json:"version_no"`
	PolicyName  string             `json:"policy_name"`
	PolicyCode  string             `json:"policy_code"`
	Description *string            `json:"description,omitempty"`
	Priority    int                `json:"priority"`
	Status      string             `json:"status"`
	Rules       []AccessPolicyRule `json:"rules"`
	Subjects    []PolicySubjectRef `json:"subjects"`
	ResourceIDs []int64            `json:"resource_ids"`
}

type PolicyGroupOption struct {
	ID          int64  `xorm:"'id'" json:"id"`
	Name        string `xorm:"'name'" json:"name"`
	Code        string `xorm:"'code'" json:"code"`
	Status      string `xorm:"'status'" json:"status"`
	MemberCount int64  `xorm:"'member_count'" json:"member_count"`
}

type PolicyGatewayOption struct {
	ID       int64  `xorm:"'id'" json:"id"`
	Name     string `xorm:"'name'" json:"name"`
	NodeCode string `xorm:"'node_code'" json:"node_code"`
	Status   string `xorm:"'status'" json:"status"`
}

// PolicyResourceOption describes an item from the enterprise protected
// resource catalog that can be assigned to an access policy.
type PolicyResourceOption struct {
	ID            int64   `json:"id"`
	VersionNo     int64   `json:"version_no"`
	Name          string  `json:"name"`
	Code          string  `json:"code"`
	Address       *string `json:"address,omitempty"`
	NetworkID     *int64  `json:"network_id,omitempty"`
	NetworkName   *string `json:"network_name,omitempty"`
	NetworkCIDR   *string `json:"network_cidr,omitempty"`
	GatewayNodeID *int64  `json:"gateway_node_id,omitempty"`
	GatewayName   *string `json:"gateway_name,omitempty"`
	Protocol      *string `json:"protocol,omitempty"`
	PortStart     *int    `json:"port_start,omitempty"`
	PortEnd       *int    `json:"port_end,omitempty"`
	Status        string  `json:"status"`
	Description   *string `json:"description,omitempty"`
	Category      *string `json:"category,omitempty"`
}

// ProtectedResourceView is the persisted enterprise resource catalog entry.
type ProtectedResourceView struct {
	ID            int64    `json:"id"`
	VersionNo     int64    `json:"version_no"`
	ResourceName  string   `json:"resource_name"`
	ResourceCode  string   `json:"resource_code"`
	ResourceType  string   `json:"resource_type"`
	NetworkID     int64    `json:"network_id"`
	NetworkName   string   `json:"network_name"`
	NetworkCIDR   string   `json:"network_cidr"`
	GatewayNodeID int64    `json:"gateway_node_id"`
	GatewayName   string   `json:"gateway_name"`
	Host          *string  `json:"host,omitempty"`
	Address       string   `json:"address"`
	PortStart     int      `json:"port_start"`
	PortEnd       int      `json:"port_end"`
	Protocol      string   `json:"protocol"`
	AccessMode    string   `json:"access_mode"`
	Category      *string  `json:"category,omitempty"`
	Tags          []string `json:"tags"`
	Description   *string  `json:"description,omitempty"`
	Status        string   `json:"status"`
	PolicyCount   int64    `json:"policy_count"`
}

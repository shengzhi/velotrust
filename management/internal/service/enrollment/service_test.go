package enrollment

import (
	"context"
	"strings"
	"testing"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/enrollment"
	ipamModel "velotrust/management/internal/model/ipam"
	"velotrust/management/internal/repository/enrollment"
)

type ownershipCatalog struct{ networks []ipamModel.Network }

func (c ownershipCatalog) ListSelectableSiteNetworks(*servicecontext.Context) ([]ipamModel.Network, error) {
	return c.networks, nil
}

func (c ownershipCatalog) ResolveActiveSiteNetworks(*servicecontext.Context, []int64) ([]ipamModel.Network, error) {
	return c.networks, nil
}

type ownershipPermissions struct{}

func (ownershipPermissions) HasPermission(context.Context, int64, string) (bool, error) {
	return true, nil
}

type ownershipStore struct {
	enrollment.Store
	called bool
}

func (s *ownershipStore) CreateGatewayAndCredential(*servicecontext.Context, int64, *model.Gateway, *model.Credential, *model.AuditEvent) error {
	s.called = true
	return nil
}

func ownershipContext() *servicecontext.Context {
	return servicecontext.New(context.Background(), servicecontext.Principal{ID: "42", Name: "operator"}, servicecontext.RequestMeta{})
}

func TestSelectableSitesExcludeNetworksOwnedByAnotherGateway(t *testing.T) {
	owner := int64(9)
	service, err := New(&ownershipStore{}, ownershipPermissions{}, ownershipCatalog{networks: []ipamModel.Network{
		{ID: 1, CIDR: "10.1.0.0/24"},
		{ID: 2, CIDR: "10.2.0.0/24", GatewayNodeID: &owner},
	}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.ListSelectableSiteNetworks(ownershipContext())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != 1 {
		t.Fatalf("expected only the unbound site, got %+v", items)
	}
}

func TestCreateGatewayRechecksSiteOwnership(t *testing.T) {
	// A caller can submit a stale selection even after the UI filtered the list.
	for _, occupied := range []bool{false, true} {
		name := "unbound"
		if occupied {
			name = "already_assigned"
		}
		t.Run(name, func(t *testing.T) {
			owner := int64(9)
			network := ipamModel.Network{ID: 1, CIDR: "10.1.0.0/24"}
			if occupied {
				network.GatewayNodeID = &owner
			}
			store := &ownershipStore{}
			service, err := New(store, ownershipPermissions{}, ownershipCatalog{networks: []ipamModel.Network{network}})
			if err != nil {
				t.Fatal(err)
			}
			grant, err := service.CreateGateway(ownershipContext(), CreateGatewayInput{
				GatewayName: "branch", GatewayType: model.GatewayTypeSite,
				NetworkName: "branch-network", AllowedNetworkIDs: []int64{1},
			})
			if occupied {
				if err == nil || !strings.Contains(err.Error(), "already assigned") || store.called || grant != nil {
					t.Fatalf("occupied site must not reach the write transaction: called=%v, err=%v", store.called, err)
				}
				return
			}
			if err != nil || !store.called || grant == nil {
				t.Fatalf("unbound site should permit registration: called=%v, err=%v", store.called, err)
			}
		})
	}
}

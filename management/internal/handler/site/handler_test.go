package site

import (
	"encoding/json"
	"testing"
)

func TestSaveSiteGatewayOwnershipContract(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
		wantID  int64
		wantErr bool
	}{
		{"assign", `{"gateway_node_id":12}`, 12, false},
		{"unbind", `{"gateway_node_id":null}`, 0, false},
		{"reject_stale_client", `{"gateway_row_id":12}`, 0, true},
		{"reject_stale_unbind", `{"gateway_row_id":null}`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var args SaveSiteArgs
			err := json.Unmarshal([]byte(tc.payload), &args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("decode error=%v, wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if tc.wantID == 0 && args.GatewayNodeID != nil {
				t.Fatal("explicit null must clear the gateway binding")
			}
			if tc.wantID != 0 && (args.GatewayNodeID == nil || *args.GatewayNodeID != tc.wantID) {
				t.Fatal("gateway_node_id was not decoded")
			}
		})
	}
}

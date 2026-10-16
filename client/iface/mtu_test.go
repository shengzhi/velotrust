package iface

import "testing"

func TestValidateMTU(t *testing.T) {
	tests := []struct {
		name    string
		mtu     uint16
		wantErr bool
	}{
		{name: "minimum", mtu: MinMTU},
		{name: "default", mtu: DefaultMTU},
		{name: "maximum", mtu: MaxMTU},
		{name: "below minimum", mtu: MinMTU - 1, wantErr: true},
		{name: "above maximum", mtu: MaxMTU + 1, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateMTU(test.mtu)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateMTU(%d) error = %v, wantErr %v", test.mtu, err, test.wantErr)
			}
		})
	}
}

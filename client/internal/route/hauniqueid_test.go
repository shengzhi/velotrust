package route

import "testing"

func TestHAUniqueIDNetID(t *testing.T) {
	for _, test := range []struct {
		value string
		want  NetID
	}{
		{value: "network-a|peer-1", want: "network-a"},
		{value: "network|with|separator|peer-1", want: "network|with|separator"},
		{value: "network-a", want: "network-a"},
	} {
		if got := HAUniqueID(test.value).NetID(); got != test.want {
			t.Errorf("HAUniqueID(%q).NetID() = %q, want %q", test.value, got, test.want)
		}
	}
}

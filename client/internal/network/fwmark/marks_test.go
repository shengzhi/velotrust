package fwmark

import "testing"

func TestParseBase(t *testing.T) {
	for _, test := range []struct {
		input string
		want  uint32
	}{
		{input: "0x5A000", want: 0x5A000},
		{input: "65536", want: 65536},
		{input: " 0o400 ", want: 0o400},
	} {
		got, err := ParseBase(test.input)
		if err != nil || got != test.want {
			t.Errorf("ParseBase(%q) = %#x, %v; want %#x", test.input, got, err, test.want)
		}
	}
	for _, input := range []string{"", "0", "0x1BD01", "-0x100", "0x1FFFFFFFF"} {
		if _, err := ParseBase(input); err == nil {
			t.Errorf("ParseBase(%q) succeeded, want error", input)
		}
	}
}

func TestMarksStayInConfiguredRange(t *testing.T) {
	if ControlPlane < base || ControlPlane > base|offsetMask {
		t.Fatalf("control-plane mark %#x is outside base %#x", ControlPlane, base)
	}
	if IsDataPlane(ControlPlane) {
		t.Fatal("control-plane mark overlaps data-plane range")
	}
	if !IsDataPlane(DataPlaneIn) || !IsDataPlane(DataPlaneOut) {
		t.Fatal("data-plane marks are outside the data-plane range")
	}
}

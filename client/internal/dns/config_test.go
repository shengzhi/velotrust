package dns

import "testing"

func TestGetParsedDomainLabel(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "node.example.test", want: "node"},
		{name: "Bücher.example", want: "xn--bcher-kva"},
		{name: "node_name.example", want: "node-name"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := GetParsedDomainLabel(test.name)
			if err != nil {
				t.Fatalf("GetParsedDomainLabel() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("GetParsedDomainLabel() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestGetParsedDomainLabelRejectsEmptyName(t *testing.T) {
	for _, name := range []string{"", "."} {
		if got, err := GetParsedDomainLabel(name); err == nil || got != "" {
			t.Errorf("GetParsedDomainLabel(%q) = %q, %v; want an error", name, got, err)
		}
	}
}

func TestNormalizeZone(t *testing.T) {
	if got := NormalizeZone("*.example.test"); got != "example.test" {
		t.Fatalf("NormalizeZone() = %q", got)
	}
	if got := NormalizeZone("example.test"); got != "example.test" {
		t.Fatalf("NormalizeZone() changed a non-wildcard zone to %q", got)
	}
}

func TestSimpleRecord(t *testing.T) {
	record := SimpleRecord{Name: "host.example.test", Type: TypeA, Class: "IN", TTL: 300, RData: "192.0.2.1"}
	if got, want := record.String(), "host.example.test. 300 IN A 192.0.2.1"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if got := record.Len(); got != 4 {
		t.Fatalf("Len() = %d, want 4", got)
	}
}

func TestConfigDoesNotRequireManagementModel(t *testing.T) {
	config := Config{
		Enabled:        true,
		ResolverGroups: []ResolverGroup{{Domains: []string{"example.test"}, Primary: true}},
		Zones:          []CustomZone{{Domain: "example.test"}},
	}
	if !config.Enabled || len(config.ResolverGroups) != 1 || len(config.Zones) != 1 {
		t.Fatalf("unexpected config: %#v", config)
	}
}

func TestParseResolverURL(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{input: "udp://1.1.1.1:53", want: "1.1.1.1:53"},
		{input: "UDP://[2001:db8::53]:5353", want: "[2001:db8::53]:5353"},
	} {
		got, err := ParseResolverURL(test.input)
		if err != nil {
			t.Errorf("ParseResolverURL(%q) error = %v", test.input, err)
			continue
		}
		if got.String() != test.want {
			t.Errorf("ParseResolverURL(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestParseResolverURLRejectsInvalidInputs(t *testing.T) {
	for _, input := range []string{
		"", "tcp://1.1.1.1:53", "udp://example.test:53", "udp://1.1.1.1:0",
		"udp://1.1.1.1:65536", "udp://user@1.1.1.1:53", "udp://1.1.1.1:53/path",
	} {
		if _, err := ParseResolverURL(input); err == nil {
			t.Errorf("ParseResolverURL(%q) succeeded, want error", input)
		}
	}
}

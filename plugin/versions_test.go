package plugin

import "testing"

func TestConstraints(t *testing.T) {
	tests := []struct {
		constraint string
		allows     []string
		rejects    []string
	}{
		{"^1.2", []string{"1.2", "1.2.9", "1.9"}, []string{"1.1.9", "2.0"}},
		{"1.2", []string{"1.2", "1.9"}, []string{"1.1", "2.0"}},
		{"^1", []string{"1.0", "1.9"}, []string{"0.9", "2.0"}},
		{"^0.2", []string{"0.2", "0.2.5"}, []string{"0.1", "0.3"}},
		{"^0.0.3", []string{"0.0.3"}, []string{"0.0.2", "0.0.4"}},
		{"^0", []string{"0.0.1", "0.9"}, []string{"1.0"}},
		{"~1.2", []string{"1.2", "1.2.7"}, []string{"1.3", "1.1"}},
		{"~1", []string{"1.0", "1.5"}, []string{"2.0"}},
		{"=1.2.3", []string{"1.2.3"}, []string{"1.2.2", "1.2.4"}},
	}

	for _, tt := range tests {
		c, err := ParseConstraint(tt.constraint)
		if err != nil {
			t.Fatalf("ParseConstraint(%q): %v", tt.constraint, err)
		}
		for _, s := range tt.allows {
			if v, _ := ParseVersion(s); !c.Allows(v) {
				t.Errorf("%s rejects %s, want it allowed", tt.constraint, s)
			}
		}
		for _, s := range tt.rejects {
			if v, _ := ParseVersion(s); c.Allows(v) {
				t.Errorf("%s allows %s, want it rejected", tt.constraint, s)
			}
		}
	}
}

func TestBadVersions(t *testing.T) {
	for _, s := range []string{"", "v1", "1.2.3.4", "1..2", "-1", "01", "1.x"} {
		if _, err := ParseVersion(s); err == nil {
			t.Errorf("ParseVersion(%q) = nil error, want one", s)
		}
	}
	for _, s := range []string{">=1.2", "^", "~v1", "*"} {
		if _, err := ParseConstraint(s); err == nil {
			t.Errorf("ParseConstraint(%q) = nil error, want one", s)
		}
	}
}

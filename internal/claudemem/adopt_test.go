package claudemem

import "testing"

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"1.2.3", "1.2.3", 0},
		{"2.0.0", "1.99.99", 1},
		{"1.0", "1.0.0", 0},
		{"1.10.0", "1.2.0", 1},
		{"0.0.1", "0.0.0", 1},
	}
	for _, tc := range tests {
		got := compareSemver(tc.a, tc.b)
		// Normalize to sign to keep the test resilient to non-{-1,0,1} returns.
		gotSign := 0
		switch {
		case got < 0:
			gotSign = -1
		case got > 0:
			gotSign = 1
		}
		if gotSign != tc.want {
			t.Errorf("compareSemver(%q, %q) = %d, want sign %d", tc.a, tc.b, got, tc.want)
		}
	}
}

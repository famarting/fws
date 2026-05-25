package cmux

import "testing"

func TestExtractUUID(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"workspace 12345678-1234-1234-1234-123456789012 created", "12345678-1234-1234-1234-123456789012"},
		{"AAAAAAAA-bbbb-CCCC-dddd-EEEEEEEEEEEE", "AAAAAAAA-bbbb-CCCC-dddd-EEEEEEEEEEEE"},
		{"no uuid here", ""},
		{"12345678-1234-1234-1234-12345678901", ""}, // 1 char short
		{"12345678-1234-1234-1234-1234567890123", "12345678-1234-1234-1234-123456789012"}, // matches the prefix that satisfies the regex
		{"", ""},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := extractUUID(tc.in); got != tc.want {
				t.Errorf("extractUUID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExtractRef(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"OK workspace:42", "workspace:42"},
		{"created workspace:N (uuid=foo)", "workspace:N"},
		{"workspace: leading space", ""},
		{"no ref here", ""},
		{"", ""},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := extractRef(tc.in); got != tc.want {
				t.Errorf("extractRef(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

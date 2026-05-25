package cli

import "testing"

func TestRemoteFromBase(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"origin/main", "origin"},
		{"upstream/master", "upstream"},
		{"origin/feature/branch-name", "origin"},
		{"main", "origin"},
		{"", "origin"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := remoteFromBase(tc.in); got != tc.want {
				t.Errorf("remoteFromBase(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestValidateBase(t *testing.T) {
	tests := []struct {
		in      string
		wantErr bool
	}{
		{"origin/main", false},
		{"main", false},
		{"origin/feature/branch", false},
		{"", true},
		{"-rf", true},
		{"--upload-pack=evil", true},
		{"-flag", true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			err := validateBase(tc.in)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateBase(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			}
		})
	}
}

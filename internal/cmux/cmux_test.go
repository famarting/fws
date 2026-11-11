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

func TestLocate(t *testing.T) {
	title := "cloudgrid: feat"
	other := "cloudgrid: other"
	wss := []Workspace{
		{ID: "AAA", CustomTitle: &other, Dir: "/wt/cloudgrid__other"},
		{ID: "BBB", CustomTitle: &title, Dir: "/elsewhere"},
		{ID: "CCC", Dir: "/wt/cloudgrid__feat/sub"},
	}
	tests := []struct {
		name, id, title, dir, want string
	}{
		{"live id wins", "aaa", title, "/wt/cloudgrid__feat", "AAA"},
		{"stale id falls back to title", "DEAD", title, "/wt/cloudgrid__feat", "BBB"},
		{"falls back to cwd inside worktree", "DEAD", "cloudgrid: renamed", "/wt/cloudgrid__feat", "CCC"},
		{"cwd prefix must be a path boundary", "DEAD", "", "/wt/cloudgrid__fe", ""},
		{"no recorded id skips cwd fallback", "", "cloudgrid: renamed", "/wt/cloudgrid__feat", ""},
		{"no recorded id still matches title", "", title, "/wt/cloudgrid__feat", "BBB"},
		{"nothing matches", "DEAD", "x: y", "/nope", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := locate(wss, tc.id, tc.title, tc.dir); got != tc.want {
				t.Errorf("locate() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUUIDForRef(t *testing.T) {
	list := "* workspace:1 3790D78D-5376-4205-93AD-4D0F9FC4DA5B  claudio  [selected]\n" +
		"  workspace:11 1CA937F6-4491-4869-A570-FFAECC23FCC0  cloudgrid: feat"
	if got := uuidForRef(list, "workspace:11"); got != "1CA937F6-4491-4869-A570-FFAECC23FCC0" {
		t.Errorf("workspace:11 = %q", got)
	}
	if got := uuidForRef(list, "workspace:12"); got != "" {
		t.Errorf("missing ref = %q, want empty", got)
	}
}

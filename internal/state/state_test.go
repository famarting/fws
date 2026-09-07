package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeName(t *testing.T) {
	for in, want := range map[string]string{
		"Fix-Auth":    "fix-auth",
		"FEAT/Foo":    "feat/foo",
		"already-low": "already-low",
	} {
		if got := NormalizeName(in); got != want {
			t.Errorf("NormalizeName(%q) = %q, want %q", in, got, want)
		}
		if err := ValidateName(NormalizeName(in)); err != nil {
			t.Errorf("ValidateName(NormalizeName(%q)) = %v", in, err)
		}
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"fix-auth", false},
		{"add-metrics-v2", false},
		{"a", false},
		{"123abc", false},
		{"abc123", false},
		{"feat/foo", false},
		{"user/feat/foo", false},
		{"", true},
		{"-leading-dash", true},
		{"Capital", true},
		{"under_score", true},
		{"with spaces", true},
		{"../escape", true},
		{"dot.name", true},
		{"/leading-slash", true},
		{"trailing-slash/", true},
		{"double//slash", true},
		{strings.Repeat("a", 50), false},
		{strings.Repeat("a", 51), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateName(tc.name)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestComposite(t *testing.T) {
	tests := []struct {
		repo, name, want string
	}{
		{"myrepo", "fix-auth", "myrepo__fix-auth"},
		{"myrepo", "feat/foo", "myrepo__feat+foo"},
		{"myrepo", "user/feat/foo", "myrepo__user+feat+foo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Composite(tc.repo, tc.name); got != tc.want {
				t.Errorf("Composite(%q, %q) = %q, want %q", tc.repo, tc.name, got, tc.want)
			}
		})
	}
}

func TestIsManagedDirName(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"myrepo__fix-auth", true},
		{"a__b", true},
		{"plain-dir", false},
		{"", false},
		{"single_underscore", false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := IsManagedDirName(tc.in); got != tc.want {
				t.Errorf("IsManagedDirName(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestWriteAndRead_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	original := Sidecar{
		Name:            "fix-auth",
		Repo:            "myrepo",
		RepoPath:        "/repos/myrepo",
		WorktreePath:    filepath.Join(dir, "myrepo__fix-auth"),
		Branch:          "fix-auth",
		Base:            "origin/main",
		BaseSHA:         "abc123",
		CmuxWorkspaceID: "some-id",
		CreatedAt:       time.Now().UTC().Truncate(time.Second),
	}
	if err := Write(dir, original); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Read(SidecarPath(dir, original.Repo, original.Name))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Name != original.Name || got.Repo != original.Repo || got.BaseSHA != original.BaseSHA {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, original)
	}
	if !got.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", got.CreatedAt, original.CreatedAt)
	}
}

func TestRead_CreatedAtFallbackToMtime(t *testing.T) {
	dir := t.TempDir()
	// Write a sidecar JSON with no created_at field.
	raw := map[string]any{
		"name":          "fix-auth",
		"repo":          "myrepo",
		"worktree_path": "/whatever",
	}
	data, _ := json.Marshal(raw)
	p := filepath.Join(dir, "myrepo__fix-auth.json")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	sc, err := Read(p)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if sc.CreatedAt.IsZero() {
		t.Error("CreatedAt should fall back to file mtime when missing from JSON")
	}
}

func TestResolvePath_AbsAndStableForExisting(t *testing.T) {
	dir := t.TempDir()
	got := ResolvePath(dir)
	if !filepath.IsAbs(got) {
		t.Errorf("ResolvePath(%q) = %q, expected absolute path", dir, got)
	}
}

func TestList_SkipsUnreadable(t *testing.T) {
	dir := t.TempDir()
	// Valid sidecar.
	good := Sidecar{Name: "good", Repo: "myrepo", CreatedAt: time.Now()}
	if err := Write(dir, good); err != nil {
		t.Fatal(err)
	}
	// Garbage JSON; List should warn-and-continue, not bail.
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	scs, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(scs) != 1 || scs[0].Name != "good" {
		t.Errorf("List returned %+v, expected the one good sidecar", scs)
	}
}

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveAllForceReadOnlyDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "wt")
	ro := filepath.Join(root, "bin", "k8s")
	if err := os.MkdirAll(ro, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ro, "etcd"), nil, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}

	if err := removeAllForce(root); err != nil {
		t.Fatalf("removeAllForce: %v", err)
	}
	if pathExists(root) {
		t.Fatalf("%s still exists", root)
	}
}

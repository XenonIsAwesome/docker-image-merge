package merge

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestTree(t *testing.T) (string, string) {
	t.Helper()

	rootA := t.TempDir()
	rootB := t.TempDir()

	// Common file, identical in both
	writeFile(t, rootA, "common.txt", "hello\n")
	writeFile(t, rootB, "common.txt", "hello\n")

	// File only in A
	writeFile(t, rootA, "only-a.txt", "from A\n")

	// File only in B
	writeFile(t, rootB, "only-b.txt", "from B\n")

	// Content conflict
	writeFile(t, rootA, "conflict.txt", "version A\n")
	writeFile(t, rootB, "conflict.txt", "version B\n")

	// Permission-only difference
	writeFile(t, rootA, "perm-file.txt", "same content\n")
	writeFile(t, rootB, "perm-file.txt", "same content\n")
	_ = os.Chmod(filepath.Join(rootA, "perm-file.txt"), 0644)
	_ = os.Chmod(filepath.Join(rootB, "perm-file.txt"), 0755)

	// Type change: file in A, dir in B
	writeFile(t, rootA, "type-change", "I am a file\n")
	_ = os.MkdirAll(filepath.Join(rootB, "type-change"), 0755)

	// Nested identical
	_ = os.MkdirAll(filepath.Join(rootA, "sub/dir"), 0755)
	_ = os.MkdirAll(filepath.Join(rootB, "sub/dir"), 0755)
	writeFile(t, rootA, "sub/dir/nested.txt", "nested\n")
	writeFile(t, rootB, "sub/dir/nested.txt", "nested\n")

	// Nested conflict
	writeFile(t, rootA, "sub/deep.txt", "A deep\n")
	writeFile(t, rootB, "sub/deep.txt", "B deep\n")

	return rootA, rootB
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file %s: %v", rel, err)
	}
}

func TestDiffEngine_Basic(t *testing.T) {
	rootA, rootB := setupTestTree(t)

	engine := NewDiffEngine(rootA, rootB)
	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(result.Conflicts) == 0 {
		t.Fatal("expected conflicts, got none")
	}

	kinds := make(map[string]ConflictKind)
	for _, c := range result.Conflicts {
		kinds[c.Path] = c.Kind
	}

	// Verify specific conflicts
	tests := []struct {
		path string
		want ConflictKind
	}{
		{"common.txt", Same},
		{"only-a.txt", OnlyA},
		{"only-b.txt", OnlyB},
		{"conflict.txt", ContentConflict},
		{"perm-file.txt", PermOnly},
		{"type-change", TypeChange},
		{"sub/dir/nested.txt", Same},
		{"sub/deep.txt", ContentConflict},
	}

	for _, tt := range tests {
		got, ok := kinds[tt.path]
		if !ok {
			t.Errorf("path %q not found in conflicts", tt.path)
			continue
		}
		if got != tt.want {
			t.Errorf("path %q: got kind %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestDiffEngine_Identical(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()

	writeFile(t, rootA, "file.txt", "same\n")
	writeFile(t, rootB, "file.txt", "same\n")

	engine := NewDiffEngine(rootA, rootB)
	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	for _, c := range result.Conflicts {
		if c.Kind != Same {
			t.Errorf("expected Same for identical trees, got %v for %s", c.Kind, c.Path)
		}
	}
}

func TestDiffEngine_EmptyTrees(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()

	engine := NewDiffEngine(rootA, rootB)
	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(result.Conflicts) != 0 {
		t.Errorf("expected no conflicts for empty trees, got %d", len(result.Conflicts))
	}
}

func TestDiffEngine_Stats(t *testing.T) {
	rootA, rootB := setupTestTree(t)

	engine := NewDiffEngine(rootA, rootB)
	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.Stats.OnlyA < 1 {
		t.Errorf("expected OnlyA >= 1, got %d", result.Stats.OnlyA)
	}
	if result.Stats.OnlyB < 1 {
		t.Errorf("expected OnlyB >= 1, got %d", result.Stats.OnlyB)
	}
	if result.Stats.Conflicts < 1 {
		t.Errorf("expected Conflicts >= 1, got %d", result.Stats.Conflicts)
	}
}

func TestConflictNeedsResolution(t *testing.T) {
	tests := []struct {
		kind ConflictKind
		want bool
	}{
		{ContentConflict, true},
		{TypeChange, true},
		{PermOnly, true},
		{OnlyA, false},
		{OnlyB, false},
		{Same, false},
		{BothDeleted, false},
	}

	for _, tt := range tests {
		c := &Conflict{Kind: tt.kind}
		if got := c.Kind.NeedsResolution(); got != tt.want {
			t.Errorf("Kind(%v).NeedsResolution() = %v, want %v", tt.kind, got, tt.want)
		}
	}
}

func TestConflictSummary(t *testing.T) {
	tests := []struct {
		kind ConflictKind
		path string
		want string
	}{
		{OnlyA, "foo.txt", "foo.txt (only in A)"},
		{OnlyB, "bar.txt", "bar.txt (only in B)"},
		{ContentConflict, "baz.txt", "baz.txt (content conflict)"},
		{PermOnly, "perm.txt", "perm.txt (permissions differ)"},
	}

	for _, tt := range tests {
		c := &Conflict{Path: tt.path, Kind: tt.kind}
		got := c.Summary()
		if got != tt.want {
			t.Errorf("Summary() = %q, want %q", got, tt.want)
		}
	}
}

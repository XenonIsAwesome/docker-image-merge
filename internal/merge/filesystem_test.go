package merge

import (
	"os"
	"path/filepath"
	"testing"
)

// --- Diff Engine tests ---

func setupTestTree(t *testing.T) (string, string) {
	t.Helper()

	rootA := t.TempDir()
	rootB := t.TempDir()

	writeFile(t, rootA, "common.txt", "hello\n")
	writeFile(t, rootB, "common.txt", "hello\n")

	writeFile(t, rootA, "only-a.txt", "from A\n")
	writeFile(t, rootB, "only-b.txt", "from B\n")

	writeFile(t, rootA, "conflict.txt", "version A\n")
	writeFile(t, rootB, "conflict.txt", "version B\n")

	writeFile(t, rootA, "perm-file.txt", "same content\n")
	writeFile(t, rootB, "perm-file.txt", "same content\n")
	_ = os.Chmod(filepath.Join(rootA, "perm-file.txt"), 0644)
	_ = os.Chmod(filepath.Join(rootB, "perm-file.txt"), 0755)

	writeFile(t, rootA, "type-change", "I am a file\n")
	_ = os.MkdirAll(filepath.Join(rootB, "type-change"), 0755)

	_ = os.MkdirAll(filepath.Join(rootA, "sub/dir"), 0755)
	_ = os.MkdirAll(filepath.Join(rootB, "sub/dir"), 0755)
	writeFile(t, rootA, "sub/dir/nested.txt", "nested\n")
	writeFile(t, rootB, "sub/dir/nested.txt", "nested\n")

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

func TestDiffEngine_SortedOutput(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()

	// Create files in reverse order to test sorting.
	writeFile(t, rootA, "z.txt", "a")
	writeFile(t, rootB, "z.txt", "b")
	writeFile(t, rootA, "a.txt", "a")
	writeFile(t, rootB, "a.txt", "b")
	writeFile(t, rootA, "m.txt", "a")
	writeFile(t, rootB, "m.txt", "b")

	engine := NewDiffEngine(rootA, rootB)
	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify conflicts are sorted by path.
	for i := 1; i < len(result.Conflicts); i++ {
		if result.Conflicts[i-1].Path >= result.Conflicts[i].Path {
			t.Errorf("conflicts not sorted: %q >= %q",
				result.Conflicts[i-1].Path, result.Conflicts[i].Path)
		}
	}
}

func TestDiffEngine_Symlinks(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()

	// Same symlink target.
	_ = os.Symlink("/usr/bin/foo", filepath.Join(rootA, "link-a"))
	_ = os.Symlink("/usr/bin/foo", filepath.Join(rootB, "link-a"))

	// Different symlink target.
	_ = os.Symlink("/usr/bin/foo", filepath.Join(rootA, "link-b"))
	_ = os.Symlink("/usr/bin/bar", filepath.Join(rootB, "link-b"))

	engine := NewDiffEngine(rootA, rootB)
	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	kinds := make(map[string]ConflictKind)
	for _, c := range result.Conflicts {
		kinds[c.Path] = c.Kind
	}

	if kinds["link-a"] != Same {
		t.Errorf("link-a: got %v, want Same", kinds["link-a"])
	}
	if kinds["link-b"] != ContentConflict {
		t.Errorf("link-b: got %v, want ContentConflict", kinds["link-b"])
	}
}

func TestDiffEngine_LargeFile(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()

	// Create files larger than 1MB to trigger hash-based comparison.
	bigA := make([]byte, 2*1024*1024)
	bigB := make([]byte, 2*1024*1024)
	for i := range bigA {
		bigA[i] = byte(i % 256)
		bigB[i] = byte(i % 256)
	}
	bigB[1000] = 0xff // differ at one byte

	_ = os.WriteFile(filepath.Join(rootA, "big.bin"), bigA, 0644)
	_ = os.WriteFile(filepath.Join(rootB, "big.bin"), bigB, 0644)

	engine := NewDiffEngine(rootA, rootB)
	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	kinds := make(map[string]ConflictKind)
	for _, c := range result.Conflicts {
		kinds[c.Path] = c.Kind
	}

	if kinds["big.bin"] != ContentConflict {
		t.Errorf("big.bin: got %v, want ContentConflict", kinds["big.bin"])
	}
}

func TestDiffEngine_DeeplyNested(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()

	deep := "a/b/c/d/e/f/g/h/i/j"
	writeFile(t, rootA, deep+"/file.txt", "A\n")
	writeFile(t, rootB, deep+"/file.txt", "B\n")

	engine := NewDiffEngine(rootA, rootB)
	result, err := engine.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	found := false
	for _, c := range result.Conflicts {
		if c.Path == deep+"/file.txt" && c.Kind == ContentConflict {
			found = true
		}
	}
	if !found {
		t.Errorf("deeply nested conflict not found")
	}
}

// --- Conflict type tests ---

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
		{TypeChange, "tc.txt", "tc.txt (type changed)"},
		{Same, "same.txt", "same.txt (identical)"},
		{BothDeleted, "gone.txt", "gone.txt (deleted in both)"},
	}

	for _, tt := range tests {
		c := &Conflict{Path: tt.path, Kind: tt.kind}
		got := c.Summary()
		if got != tt.want {
			t.Errorf("Summary() = %q, want %q", got, tt.want)
		}
	}
}

func TestConflictKindString(t *testing.T) {
	tests := []struct {
		kind ConflictKind
		want string
	}{
		{OnlyA, "only-a"},
		{OnlyB, "only-b"},
		{Same, "same"},
		{ContentConflict, "conflict"},
		{TypeChange, "type-change"},
		{PermOnly, "perm-only"},
		{BothDeleted, "both-deleted"},
		{ConflictKind(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.kind.String()
		if got != tt.want {
			t.Errorf("ConflictKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestResolutionString(t *testing.T) {
	tests := []struct {
		res  Resolution
		want string
	}{
		{ResolutionNone, "unresolved"},
		{ResolutionTakeA, "take-a"},
		{ResolutionTakeB, "take-b"},
		{ResolutionSkip, "skip"},
		{Resolution(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.res.String()
		if got != tt.want {
			t.Errorf("Resolution(%d).String() = %q, want %q", tt.res, got, tt.want)
		}
	}
}

// --- Binary detection tests ---

func TestIsBinary_NullByte(t *testing.T) {
	if !isBinary([]byte("hello\x00world")) {
		t.Error("expected binary detection for null byte")
	}
}

func TestIsBinary_PlainText(t *testing.T) {
	if isBinary([]byte("hello world\n")) {
		t.Error("expected non-binary for plain text")
	}
}

func TestIsBinary_Empty(t *testing.T) {
	if isBinary([]byte{}) {
		t.Error("expected non-binary for empty data")
	}
}

func TestIsBinary_LargeNullOffset(t *testing.T) {
	// Null byte past position 512 should not trigger detection.
	data := make([]byte, 1024)
	for i := range data {
		data[i] = 'a'
	}
	data[600] = 0
	if isBinary(data) {
		t.Error("expected non-binary for null byte past 512 bytes")
	}
}

// --- Hex dump tests ---

func TestHexDump(t *testing.T) {
	data := []byte("Hello, World!\x00\x01\x02")
	dump := hexDump(data, "test")

	if len(dump) == 0 {
		t.Fatal("hexDump returned empty string")
	}
	// Should contain the label.
	if !containsStr(dump, "test") {
		t.Error("hexDump missing label")
	}
	// Should contain hex representation.
	if !containsStr(dump, "48 65 6c 6c") {
		t.Error("hexDump missing hex bytes")
	}
	// Should contain ASCII representation.
	if !containsStr(dump, "Hello") {
		t.Error("hexDump missing ASCII representation")
	}
}

func TestHexDump_Truncation(t *testing.T) {
	// Create data larger than 16 lines * 16 bytes = 256 bytes.
	data := make([]byte, 512)
	for i := range data {
		data[i] = byte(i % 256)
	}
	dump := hexDump(data, "big")
	if !containsStr(dump, "more bytes") {
		t.Error("hexDump should indicate truncation for large data")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- GenerateDiff tests ---

func TestGenerateDiff_TextFiles(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.txt")
	pathB := filepath.Join(dir, "b.txt")
	writeFile(t, dir, "a.txt", "line1\nline2\nline3\n")
	writeFile(t, dir, "b.txt", "line1\nchanged\nline3\n")

	diff := GenerateDiff(pathA, pathB, "imgA", "imgB")
	if !containsStr(diff, "imgA") {
		t.Error("diff missing label A")
	}
	if !containsStr(diff, "imgB") {
		t.Error("diff missing label B")
	}
	if !containsStr(diff, "line1") {
		t.Error("diff missing content")
	}
}

func TestGenerateDiff_BinaryFiles(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.bin")
	pathB := filepath.Join(dir, "b.bin")

	binA := []byte{0x7f, 0x45, 0x4c, 0x46, 0x00, 0x01, 0x02}
	binB := []byte{0x7f, 0x45, 0x4c, 0x46, 0x00, 0x03, 0x04}
	_ = os.WriteFile(pathA, binA, 0644)
	_ = os.WriteFile(pathB, binB, 0644)

	diff := GenerateDiff(pathA, pathB, "imgA", "imgB")
	if !containsStr(diff, "imgA") {
		t.Error("diff missing label A")
	}
	// Should contain hex representation.
	if !containsStr(diff, "7f 45 4c") {
		t.Error("diff missing hex bytes")
	}
}

func TestGenerateDiff_MissingFile(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.txt")
	writeFile(t, dir, "a.txt", "content\n")

	diff := GenerateDiff(pathA, "/nonexistent", "imgA", "imgB")
	if !containsStr(diff, "unable to read") {
		t.Error("diff should indicate missing file")
	}
}

// --- ApplyResolutions tests ---

func TestApplyResolutions_AllFromA(t *testing.T) {
	rootA, rootB := setupTestTree(t)
	tmpDir := t.TempDir()

	conflicts := []*Conflict{
		{Path: "conflict.txt", Kind: ContentConflict, Resolution: ResolutionTakeA},
		{Path: "perm-file.txt", Kind: PermOnly, Resolution: ResolutionTakeA},
		{Path: "type-change", Kind: TypeChange, Resolution: ResolutionTakeA},
	}

	result, err := ApplyResolutions(rootA, rootB, conflicts, tmpDir)
	if err != nil {
		t.Fatalf("ApplyResolutions() error: %v", err)
	}

	if result.FromA == 0 {
		t.Error("expected FromA > 0")
	}

	// Verify conflict.txt has A's content.
	data, _ := os.ReadFile(filepath.Join(result.MergedDir, "conflict.txt"))
	if string(data) != "version A\n" {
		t.Errorf("conflict.txt: got %q, want %q", string(data), "version A\n")
	}
}

func TestApplyResolutions_AllFromB(t *testing.T) {
	rootA, rootB := setupTestTree(t)
	tmpDir := t.TempDir()

	conflicts := []*Conflict{
		{Path: "conflict.txt", Kind: ContentConflict, Resolution: ResolutionTakeB},
		{Path: "perm-file.txt", Kind: PermOnly, Resolution: ResolutionTakeB},
		{Path: "type-change", Kind: TypeChange, Resolution: ResolutionTakeB},
	}

	result, err := ApplyResolutions(rootA, rootB, conflicts, tmpDir)
	if err != nil {
		t.Fatalf("ApplyResolutions() error: %v", err)
	}

	if result.FromB == 0 {
		t.Error("expected FromB > 0")
	}

	// Verify conflict.txt has B's content.
	data, _ := os.ReadFile(filepath.Join(result.MergedDir, "conflict.txt"))
	if string(data) != "version B\n" {
		t.Errorf("conflict.txt: got %q, want %q", string(data), "version B\n")
	}
}

func TestApplyResolutions_Skip(t *testing.T) {
	rootA, rootB := setupTestTree(t)
	tmpDir := t.TempDir()

	conflicts := []*Conflict{
		{Path: "conflict.txt", Kind: ContentConflict, Resolution: ResolutionSkip},
	}

	result, err := ApplyResolutions(rootA, rootB, conflicts, tmpDir)
	if err != nil {
		t.Fatalf("ApplyResolutions() error: %v", err)
	}

	if result.Skipped == 0 {
		t.Error("expected Skipped > 0")
	}

	// conflict.txt should not exist in merged (skipped).
	if _, err := os.Stat(filepath.Join(result.MergedDir, "conflict.txt")); err == nil {
		t.Error("skipped file should not exist in merged dir")
	}
}

func TestApplyResolutions_OnlyBFiles(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	writeFile(t, rootB, "b-only.txt", "from B\n")
	tmpDir := t.TempDir()

	result, err := ApplyResolutions(rootA, rootB, nil, tmpDir)
	if err != nil {
		t.Fatalf("ApplyResolutions() error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(result.MergedDir, "b-only.txt"))
	if string(data) != "from B\n" {
		t.Errorf("b-only.txt: got %q, want %q", string(data), "from B\n")
	}
}

func TestApplyResolutions_PreservesPerms(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	writeFile(t, rootA, "perm.txt", "content\n")
	_ = os.Chmod(filepath.Join(rootA, "perm.txt"), 0755)
	tmpDir := t.TempDir()

	result, err := ApplyResolutions(rootA, rootB, nil, tmpDir)
	if err != nil {
		t.Fatalf("ApplyResolutions() error: %v", err)
	}

	info, _ := os.Stat(filepath.Join(result.MergedDir, "perm.txt"))
	if info.Mode().Perm() != 0755 {
		t.Errorf("perm: got %o, want 0755", info.Mode().Perm())
	}
}

func TestApplyResolutions_PreservesSymlinks(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	_ = os.Symlink("/usr/bin/foo", filepath.Join(rootA, "link"))
	tmpDir := t.TempDir()

	result, err := ApplyResolutions(rootA, rootB, nil, tmpDir)
	if err != nil {
		t.Fatalf("ApplyResolutions() error: %v", err)
	}

	target, err := os.Readlink(filepath.Join(result.MergedDir, "link"))
	if err != nil {
		t.Fatalf("Readlink error: %v", err)
	}
	if target != "/usr/bin/foo" {
		t.Errorf("symlink target: got %q, want %q", target, "/usr/bin/foo")
	}
}

func TestApplyResolutions_EmptyTrees(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	tmpDir := t.TempDir()

	result, err := ApplyResolutions(rootA, rootB, nil, tmpDir)
	if err != nil {
		t.Fatalf("ApplyResolutions() error: %v", err)
	}

	if result.TotalFiles != 0 {
		t.Errorf("expected 0 files, got %d", result.TotalFiles)
	}
}

func TestBuildDockerfileContent(t *testing.T) {
	df := BuildDockerfileContent("alpine:latest", "/tmp/merged/", []string{"ENV FOO=bar", "CMD [\"echo\"]"})
	if !containsStr(df, "FROM alpine:latest") {
		t.Error("missing FROM")
	}
	if !containsStr(df, "COPY /tmp/merged/ /") {
		t.Error("missing COPY")
	}
	if !containsStr(df, "ENV FOO=bar") {
		t.Error("missing ENV")
	}
	if !containsStr(df, "CMD") {
		t.Error("missing CMD")
	}
}

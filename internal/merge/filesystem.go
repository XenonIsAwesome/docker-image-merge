package merge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cespare/xxhash/v2"
)

// DiffEngine compares two extracted image filesystems and produces a list of
// conflicts. It is stateless — create one per comparison and call Run().
type DiffEngine struct {
	// RootA is the absolute path to image A's extracted filesystem.
	RootA string

	// RootB is the absolute path to image B's extracted filesystem.
	RootB string
}

// DiffResult holds the complete output of a filesystem comparison.
type DiffResult struct {
	// Conflicts is the sorted list of all paths that differ between the images.
	Conflicts []*Conflict

	// Stats provides aggregate counts by conflict kind.
	Stats DiffStats
}

// DiffStats is a set of counters for each classification the diff engine produces.
type DiffStats struct {
	OnlyA       int
	OnlyB       int
	Same        int
	Conflicts   int
	TypeChange  int
	PermOnly    int
	BothDeleted int
	Total       int
}

// NewDiffEngine creates a DiffEngine that will compare the two given roots.
func NewDiffEngine(rootA, rootB string) *DiffEngine {
	return &DiffEngine{RootA: rootA, RootB: rootB}
}

// Run walks both directory trees simultaneously and returns a DiffResult
// containing every classified path and aggregate statistics.
func (d *DiffEngine) Run() (*DiffResult, error) {
	result := &DiffResult{}

	if err := d.walk("", result); err != nil {
		return nil, fmt.Errorf("walking trees: %w", err)
	}

	// Sort conflicts by path for stable, predictable output.
	sort.Slice(result.Conflicts, func(i, j int) bool {
		return result.Conflicts[i].Path < result.Conflicts[j].Path
	})

	return result, nil
}

// walk recursively compares relPath in both trees, appending results to result.
func (d *DiffEngine) walk(relPath string, result *DiffResult) error {
	pathA := filepath.Join(d.RootA, relPath)
	pathB := filepath.Join(d.RootB, relPath)

	infoA, errA := os.Lstat(pathA)
	infoB, errB := os.Lstat(pathB)

	existsA := errA == nil
	existsB := errB == nil

	// Case 1: path missing from both sides.
	switch {
	case !existsA && !existsB:
		result.Stats.BothDeleted++
		result.Conflicts = append(result.Conflicts, &Conflict{
			Path: relPath,
			Kind: BothDeleted,
		})
		return nil

	// Case 2: exists only in A.
	case existsA && !existsB:
		if infoA.IsDir() {
			return nil // directories that only exist in A are not interesting
		}
		result.Stats.OnlyA++
		result.Conflicts = append(result.Conflicts, &Conflict{
			Path: relPath,
			Kind: OnlyA,
		})
		return nil

	// Case 3: exists only in B.
	case !existsA && existsB:
		if infoB.IsDir() {
			return nil
		}
		result.Stats.OnlyB++
		result.Conflicts = append(result.Conflicts, &Conflict{
			Path: relPath,
			Kind: OnlyB,
		})
		return nil
	}

	// Both sides exist. If both are directories, recurse into children.
	if infoA.IsDir() && infoB.IsDir() {
		return d.mergeDirs(relPath, result)
	}

	// Type mismatch (file vs dir vs symlink etc.).
	if infoA.IsDir() != infoB.IsDir() {
		result.Stats.TypeChange++
		result.Conflicts = append(result.Conflicts, &Conflict{
			Path:  relPath,
			Kind:  TypeChange,
			InfoA: fileInfoFrom(pathA, relPath, infoA),
			InfoB: fileInfoFrom(pathB, relPath, infoB),
		})
		return nil
	}

	// Build FileInfo for both sides (computes content hashes for files).
	finfoA, err := d.buildFileInfo(pathA, relPath, infoA)
	if err != nil {
		return err
	}
	finfoB, err := d.buildFileInfo(pathB, relPath, infoB)
	if err != nil {
		return err
	}

	// Compare symlinks by target path.
	isSymlinkA := infoA.Mode()&fs.ModeSymlink != 0
	isSymlinkB := infoB.Mode()&fs.ModeSymlink != 0
	if isSymlinkA && isSymlinkB {
		targetA, _ := os.Readlink(pathA)
		targetB, _ := os.Readlink(pathB)
		if targetA == targetB {
			result.Stats.Same++
			result.Conflicts = append(result.Conflicts, &Conflict{
				Path:  relPath,
				Kind:  Same,
				InfoA: finfoA,
				InfoB: finfoB,
			})
			return nil
		}
		result.Stats.Conflicts++
		result.Conflicts = append(result.Conflicts, &Conflict{
			Path:  relPath,
			Kind:  ContentConflict,
			InfoA: finfoA,
			InfoB: finfoB,
		})
		return nil
	}

	// Compare file contents using hashing.
	contentSame, err := compareContent(pathA, pathB, infoA.Size())
	if err != nil {
		return fmt.Errorf("comparing %s: %w", relPath, err)
	}

	if contentSame {
		// Content is identical — check if permissions differ.
		permA := infoA.Mode().Perm()
		permB := infoB.Mode().Perm()
		if permA != permB {
			result.Stats.PermOnly++
			result.Conflicts = append(result.Conflicts, &Conflict{
				Path:  relPath,
				Kind:  PermOnly,
				InfoA: finfoA,
				InfoB: finfoB,
			})
		} else {
			result.Stats.Same++
			result.Conflicts = append(result.Conflicts, &Conflict{
				Path:  relPath,
				Kind:  Same,
				InfoA: finfoA,
				InfoB: finfoB,
			})
		}
		return nil
	}

	// Content differs — this is a real conflict.
	result.Stats.Conflicts++
	result.Conflicts = append(result.Conflicts, &Conflict{
		Path:  relPath,
		Kind:  ContentConflict,
		InfoA: finfoA,
		InfoB: finfoB,
	})
	return nil
}

// mergeDirs recurses into a directory that exists in both trees.
func (d *DiffEngine) mergeDirs(relPath string, result *DiffResult) error {
	pathA := filepath.Join(d.RootA, relPath)
	pathB := filepath.Join(d.RootB, relPath)

	entriesA, err := readDirNames(pathA)
	if err != nil {
		return err
	}
	entriesB, err := readDirNames(pathB)
	if err != nil {
		return err
	}

	// Merge the two entry lists and iterate in sorted order.
	allEntries := unionSorted(entriesA, entriesB)

	for _, name := range allEntries {
		childRel := name
		if relPath != "" {
			childRel = relPath + "/" + name
		}
		if err := d.walk(childRel, result); err != nil {
			return err
		}
	}
	return nil
}

// compareContent checks whether two files have the same content. For small
// files (< 1 MB) it reads both directly; for larger files it compares hashes.
func compareContent(pathA, pathB string, size int64) (bool, error) {
	if size < 1024*1024 {
		return compareContentDirect(pathA, pathB)
	}
	return compareContentHash(pathA, pathB)
}

// compareContentDirect reads two files in chunks and returns true if every
// byte is identical.
func compareContentDirect(pathA, pathB string) (bool, error) {
	fa, err := os.Open(pathA)
	if err != nil {
		return false, err
	}
	defer fa.Close()

	fb, err := os.Open(pathB)
	if err != nil {
		return false, err
	}
	defer fb.Close()

	bufA := make([]byte, 64*1024)
	bufB := make([]byte, 64*1024)

	for {
		nA, errA := fa.Read(bufA)
		nB, errB := fb.Read(bufB)

		if nA != nB {
			return false, nil
		}
		if nA == 0 {
			if errA == io.EOF && errB == io.EOF {
				return true, nil
			}
			if errA != io.EOF || errB != io.EOF {
				return false, nil
			}
			continue
		}

		if !bytes.Equal(bufA[:nA], bufB[:nB]) {
			return false, nil
		}

		if errA == io.EOF && errB == io.EOF {
			return true, nil
		}
		if errA == io.EOF || errB == io.EOF {
			return false, nil
		}
	}
}

// compareContentHash computes xxhash digests of both files and compares them.
// This is faster than sha256 and sufficient for equality checking.
func compareContentHash(pathA, pathB string) (bool, error) {
	hashA, err := fileHash(pathA)
	if err != nil {
		return false, err
	}
	hashB, err := fileHash(pathB)
	if err != nil {
		return false, err
	}
	return hashA == hashB, nil
}

// fileHash returns the hex-encoded xxhash of a file's content.
func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := xxhash.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// contentHash returns the hex-encoded SHA-256 of a file's content. This is
// used for producing stable hashes when needed.
func contentHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fileInfoFrom builds a FileInfo from an already-lstat'd os.FileInfo. It does
// not compute content hashes — use buildFileInfo for that.
func fileInfoFrom(absPath, relPath string, info fs.FileInfo) *FileInfo {
	fi := &FileInfo{
		RelPath: relPath,
		AbsPath: absPath,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		Mode:    uint32(info.Mode()),
	}

	fi.UID, fi.GID = getUIDGID(info)

	if info.Mode()&fs.ModeSymlink != 0 {
		fi.IsSymlink = true
		target, err := os.Readlink(absPath)
		if err == nil {
			fi.SymlinkTarget = target
		}
	}

	return fi
}

// buildFileInfo extends fileInfoFrom by also computing a content hash for
// regular (non-directory, non-symlink) files.
func (d *DiffEngine) buildFileInfo(absPath, relPath string, info fs.FileInfo) (*FileInfo, error) {
	fi := fileInfoFrom(absPath, relPath, info)

	if !info.IsDir() && info.Mode()&fs.ModeSymlink == 0 {
		hash, err := contentHash(absPath)
		if err != nil {
			return nil, err
		}
		fi.ContentHash = hash
	}

	return fi, nil
}

// readDirNames returns the sorted list of entry names in a directory,
// excluding "." and "..".
func readDirNames(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

// unionSorted returns the sorted union of two string slices with no duplicates.
// Directories are sorted before regular files for deterministic walk order.
func unionSorted(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	var result []string

	for _, name := range a {
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}
	for _, name := range b {
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		di := isDir(filepath.Join(".", result[i]))
		dj := isDir(filepath.Join(".", result[j]))
		if di != dj {
			return di
		}
		return result[i] < result[j]
	})

	return result
}

// isDir returns true if the given path exists and is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isBinary checks if the first 512 bytes of a file contain a null byte,
// which is the standard heuristic for detecting binary content.
func isBinary(data []byte) bool {
	n := len(data)
	if n > 512 {
		n = 512
	}
	for _, b := range data[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

// hexDump produces a hex+ASCII view of binary data, showing up to maxLines
// lines of 16 bytes each. This is useful for inspecting binary files in the TUI.
func hexDump(data []byte, label string) string {
	const bytesPerLine = 16
	const maxLines = 16

	n := len(data)
	lines := n / bytesPerLine
	if n%bytesPerLine != 0 {
		lines++
	}
	if lines > maxLines {
		lines = maxLines
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("--- %s (%d bytes) ---\n", label, n))

	for i := 0; i < lines; i++ {
		off := i * bytesPerLine
		end := off + bytesPerLine
		if end > n {
			end = n
		}
		chunk := data[off:end]

		// Offset
		out.WriteString(fmt.Sprintf("%08x  ", off))

		// Hex bytes
		for j := 0; j < bytesPerLine; j++ {
			if j < len(chunk) {
				out.WriteString(fmt.Sprintf("%02x ", chunk[j]))
			} else {
				out.WriteString("   ")
			}
			if j == 7 {
				out.WriteString(" ")
			}
		}

		// ASCII representation
		out.WriteString(" |")
		for _, b := range chunk {
			if b >= 0x20 && b < 0x7f {
				out.WriteByte(b)
			} else {
				out.WriteByte('.')
			}
		}
		out.WriteString("|\n")
	}

	if n > maxLines*bytesPerLine {
		out.WriteString(fmt.Sprintf("... (%d more bytes)\n", n-maxLines*bytesPerLine))
	}

	return out.String()
}

// GenerateDiff produces a simple side-by-side text representation of two files.
// The output is intended for display in the TUI diff pane.
func GenerateDiff(pathA, pathB string) string {
	fa, errA := os.ReadFile(pathA)
	fb, errB := os.ReadFile(pathB)

	if errA != nil && errB != nil {
		return "(unable to read either version)"
	}
	if errA != nil {
		return fmt.Sprintf("(unable to read A: %v)\n--- B ---\n%s", errA, string(fb))
	}
	if errB != nil {
		return fmt.Sprintf("--- A ---\n%s\n(unable to read B: %v)", string(fa), errB)
	}

	// Detect binary files and show a hex dump instead of garbled output.
	if isBinary(fa) || isBinary(fb) {
		var out strings.Builder
		out.WriteString(hexDump(fa, "Image A"))
		out.WriteString("\n")
		out.WriteString(hexDump(fb, "Image B"))
		return out.String()
	}

	linesA := strings.Split(string(fa), "\n")
	linesB := strings.Split(string(fb), "\n")

	var out strings.Builder
	out.WriteString("--- Image A ---\n")
	for i, line := range linesA {
		out.WriteString(fmt.Sprintf("%3d | %s\n", i+1, line))
	}
	out.WriteString("--- Image B ---\n")
	for i, line := range linesB {
		out.WriteString(fmt.Sprintf("%3d | %s\n", i+1, line))
	}

	return out.String()
}

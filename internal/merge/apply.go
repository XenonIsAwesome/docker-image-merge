package merge

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ApplyResult holds the outcome of applying conflict resolutions to produce
// the merged directory tree.
type ApplyResult struct {
	// MergedDir is the absolute path to the directory containing the merged filesystem.
	MergedDir string

	// TotalFiles is the number of files/directories written to the merged tree.
	TotalFiles int

	// FromA is how many files were taken from image A.
	FromA int

	// FromB is how many files were taken from image B.
	FromB int

	// Skipped is how many conflicts were skipped (kept A by default).
	Skipped int
}

// ApplyResolutions merges the two filesystem trees according to the resolution
// choices stored in each Conflict. It returns an ApplyResult describing what
// was written to a new directory inside tmpDir.
//
// The algorithm starts with image A as the base, overlays image B's unique
// files, and then applies per-file resolution choices for conflicts.
func ApplyResolutions(rootA, rootB string, conflicts []*Conflict, tmpDir string) (*ApplyResult, error) {
	mergedDir := filepath.Join(tmpDir, "merged")
	if err := os.MkdirAll(mergedDir, 0755); err != nil {
		return nil, fmt.Errorf("creating merged dir: %w", err)
	}

	// Build a lookup map for fast resolution access by relative path.
	resolutions := make(map[string]Resolution)
	for _, c := range conflicts {
		resolutions[c.Path] = c.Resolution
	}

	result := &ApplyResult{MergedDir: mergedDir}

	if err := applyWalk(rootA, rootB, mergedDir, "", resolutions, result); err != nil {
		return nil, err
	}

	return result, nil
}

// applyWalk recursively merges a single path from both trees into mergedDir,
// consulting the resolutions map for conflict decisions.
func applyWalk(rootA, rootB, mergedDir, relPath string, resolutions map[string]Resolution, result *ApplyResult) error {
	pathA := filepath.Join(rootA, relPath)
	pathB := filepath.Join(rootB, relPath)
	mergedPath := filepath.Join(mergedDir, relPath)

	infoA, errA := os.Lstat(pathA)
	infoB, errB := os.Lstat(pathB)

	existsA := errA == nil
	existsB := errB == nil

	res, hasResolution := resolutions[relPath]

	switch {
	// Only in B: copy from B.
	case !existsA && existsB:
		if infoB.IsDir() {
			return copyDir(pathB, mergedPath)
		}
		if err := copyFilePreserve(pathB, mergedPath); err != nil {
			return err
		}
		result.FromB++
		result.TotalFiles++
		return nil

	// Only in A: copy from A.
	case existsA && !existsB:
		if infoA.IsDir() {
			return copyDir(pathA, mergedPath)
		}
		if err := copyFilePreserve(pathA, mergedPath); err != nil {
			return err
		}
		result.FromA++
		result.TotalFiles++
		return nil

	// Both sides exist.
	case existsA && existsB:
		// Both directories: recurse into children.
		if infoA.IsDir() && infoB.IsDir() {
			return applyMergeDirs(rootA, rootB, mergedDir, relPath, resolutions, result)
		}

		// Type mismatch (file vs dir): use resolution to pick one.
		if infoA.IsDir() || infoB.IsDir() {
			if hasResolution && res == ResolutionTakeB {
				if infoB.IsDir() {
					return copyDir(pathB, mergedPath)
				}
				if err := copyFilePreserve(pathB, mergedPath); err != nil {
					return err
				}
				result.FromB++
			} else {
				if infoA.IsDir() {
					return copyDir(pathA, mergedPath)
				}
				if err := copyFilePreserve(pathA, mergedPath); err != nil {
					return err
				}
				result.FromA++
			}
			result.TotalFiles++
			return nil
		}

		// Both are regular files (or symlinks): apply the resolution.
		if !hasResolution || res == ResolutionTakeA || res == ResolutionNone {
			if err := copyFilePreserve(pathA, mergedPath); err != nil {
				return err
			}
			result.FromA++
		} else if res == ResolutionTakeB {
			if err := copyFilePreserve(pathB, mergedPath); err != nil {
				return err
			}
			result.FromB++
		} else if res == ResolutionSkip {
			result.Skipped++
			return nil
		}
		result.TotalFiles++
		return nil
	}

	return nil
}

// applyMergeDirs recurses into a directory that exists in both trees,
// processing every child entry.
func applyMergeDirs(rootA, rootB, mergedDir, relPath string, resolutions map[string]Resolution, result *ApplyResult) error {
	pathA := filepath.Join(rootA, relPath)
	pathB := filepath.Join(rootB, relPath)

	entriesA, _ := readDirNames(pathA)
	entriesB, _ := readDirNames(pathB)
	allEntries := unionSorted(entriesA, entriesB)

	for _, name := range allEntries {
		childRel := name
		if relPath != "" {
			childRel = relPath + "/" + name
		}
		if err := applyWalk(rootA, rootB, mergedDir, childRel, resolutions, result); err != nil {
			return err
		}
	}
	return nil
}

// copyFilePreserve copies a single file from src to dst, preserving
// permissions, ownership, xattrs, and symlink targets.
func copyFilePreserve(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// Handle symlinks: recreate the link in the destination.
	linkTarget, err := os.Readlink(src)
	if err == nil {
		return os.Symlink(linkTarget, dst)
	}

	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	// Skip named pipes and other special files — they can't be meaningfully copied.
	if info.Mode()&os.ModeType == os.ModeNamedPipe {
		return nil
	}

	// Copy file content.
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	if err := dstFile.Close(); err != nil {
		return err
	}

	// Restore original permissions.
	if err := os.Chmod(dst, info.Mode()); err != nil {
		return err
	}

	// Restore ownership if running as root.
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		_ = os.Chown(dst, int(stat.Uid), int(stat.Gid))
	}

	return copyXattrs(src, dst)
}

// copyDir recursively copies an entire directory tree from src to dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFilePreserve(path, dstPath)
	})
}

// copyXattrs is a placeholder for extended attribute copying. Currently a no-op
// because xattr support varies by platform and most image merges don't need it.
func copyXattrs(src, dst string) error {
	return nil
}

// BuildDockerfileContent generates a minimal Dockerfile that layers the merged
// filesystem on top of a base image. Used by the layered build path.
func BuildDockerfileContent(baseImage, copiedFiles string, changes []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("FROM %s\n", baseImage))

	if copiedFiles != "" {
		b.WriteString(fmt.Sprintf("COPY %s /\n", copiedFiles))
	}

	for _, change := range changes {
		b.WriteString(fmt.Sprintf("%s\n", change))
	}

	return b.String()
}

package merge

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type ApplyResult struct {
	MergedDir  string
	TotalFiles int
	FromA      int
	FromB      int
	Skipped    int
}

func ApplyResolutions(rootA, rootB string, conflicts []*Conflict, tmpDir string) (*ApplyResult, error) {
	mergedDir := filepath.Join(tmpDir, "merged")
	if err := os.MkdirAll(mergedDir, 0755); err != nil {
		return nil, fmt.Errorf("creating merged dir: %w", err)
	}

	resolutions := make(map[string]Resolution)
	for _, c := range conflicts {
		resolutions[c.Path] = c.Resolution
	}

	result := &ApplyResult{MergedDir: mergedDir}

	err := applyWalk(rootA, rootB, mergedDir, "", resolutions, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

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

	case existsA && existsB:
		if infoA.IsDir() && infoB.IsDir() {
			return applyMergeDirs(rootA, rootB, mergedDir, relPath, resolutions, result)
		}
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

func copyFilePreserve(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	linkTarget, err := os.Readlink(src)
	if err == nil {
		return os.Symlink(linkTarget, dst)
	}

	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if info.Mode()&os.ModeType == os.ModeNamedPipe {
		return nil
	}

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

	if err := os.Chmod(dst, info.Mode()); err != nil {
		return err
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		_ = os.Chown(dst, int(stat.Uid), int(stat.Gid))
	}

	return copyXattrs(src, dst)
}

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

func copyXattrs(src, dst string) error {
	return nil
}

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

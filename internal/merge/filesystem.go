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
	"syscall"

	"github.com/cespare/xxhash/v2"
)

type DiffEngine struct {
	RootA string
	RootB string
}

type DiffResult struct {
	Conflicts []*Conflict
	Stats     DiffStats
}

type DiffStats struct {
	OnlyA      int
	OnlyB      int
	Same       int
	Conflicts  int
	TypeChange int
	PermOnly   int
	BothDeleted int
	Total      int
}

func NewDiffEngine(rootA, rootB string) *DiffEngine {
	return &DiffEngine{RootA: rootA, RootB: rootB}
}

func (d *DiffEngine) Run() (*DiffResult, error) {
	result := &DiffResult{}

	err := d.walk("", result)
	if err != nil {
		return nil, fmt.Errorf("walking trees: %w", err)
	}

	sort.Slice(result.Conflicts, func(i, j int) bool {
		return result.Conflicts[i].Path < result.Conflicts[j].Path
	})

	return result, nil
}

func (d *DiffEngine) walk(relPath string, result *DiffResult) error {
	pathA := filepath.Join(d.RootA, relPath)
	pathB := filepath.Join(d.RootB, relPath)

	infoA, errA := os.Lstat(pathA)
	infoB, errB := os.Lstat(pathB)

	existsA := errA == nil
	existsB := errB == nil

	switch {
	case !existsA && !existsB:
		result.Stats.BothDeleted++
		result.Conflicts = append(result.Conflicts, &Conflict{
			Path: relPath,
			Kind: BothDeleted,
		})
		return nil

	case existsA && !existsB:
		if infoA.IsDir() {
			return nil
		}
		result.Stats.OnlyA++
		result.Conflicts = append(result.Conflicts, &Conflict{
			Path: relPath,
			Kind: OnlyA,
		})
		return nil

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

	if infoA.IsDir() && infoB.IsDir() {
		return d.mergeDirs(relPath, result)
	}

	if infoA.IsDir() != infoB.IsDir() {
		result.Stats.TypeChange++
		result.Conflicts = append(result.Conflicts, &Conflict{
			Path: relPath,
			Kind: TypeChange,
			InfoA: fileInfoFrom(pathA, relPath, infoA),
			InfoB: fileInfoFrom(pathB, relPath, infoB),
		})
		return nil
	}

	finfoA, err := d.buildFileInfo(pathA, relPath, infoA)
	if err != nil {
		return err
	}
	finfoB, err := d.buildFileInfo(pathB, relPath, infoB)
	if err != nil {
		return err
	}

	isSymlinkA := infoA.Mode()&fs.ModeSymlink != 0
	isSymlinkB := infoB.Mode()&fs.ModeSymlink != 0
	if isSymlinkA && isSymlinkB {
		targetA, _ := os.Readlink(pathA)
		targetB, _ := os.Readlink(pathB)
		if targetA == targetB {
			result.Stats.Same++
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

	contentSame, err := compareContent(pathA, pathB, infoA.Size())
	if err != nil {
		return fmt.Errorf("comparing %s: %w", relPath, err)
	}

	if contentSame {
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
		}
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

func compareContent(pathA, pathB string, size int64) (bool, error) {
	if size < 1024*1024 {
		return compareContentDirect(pathA, pathB)
	}
	return compareContentHash(pathA, pathB)
}

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

func fileInfoFrom(absPath, relPath string, info fs.FileInfo) *FileInfo {
	fi := &FileInfo{
		RelPath: relPath,
		AbsPath: absPath,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		Mode:    uint32(info.Mode()),
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		fi.UID = int(stat.Uid)
		fi.GID = int(stat.Gid)
	}

	if info.Mode()&fs.ModeSymlink != 0 {
		fi.IsSymlink = true
		target, err := os.Readlink(absPath)
		if err == nil {
			fi.SymlinkTarget = target
		}
	}

	return fi
}

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

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

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

//go:build !linux

package merge

import "io/fs"

// getUIDGID returns (0, 0) on non-Linux platforms where syscall.Stat_t
// is not available.
func getUIDGID(info fs.FileInfo) (int, int) {
	return 0, 0
}

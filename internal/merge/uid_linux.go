//go:build linux

package merge

import (
	"io/fs"
	"syscall"
)

// getUIDGID extracts the numeric UID and GID from a file's underlying
// syscall.Stat_t on Linux. Returns (0, 0) if the assertion fails.
func getUIDGID(info fs.FileInfo) (int, int) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return int(stat.Uid), int(stat.Gid)
	}
	return 0, 0
}

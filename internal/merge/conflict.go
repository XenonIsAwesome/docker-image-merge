// Package merge provides the core diff engine, conflict types, and filesystem
// merge logic for docker-image-merge.
//
// The diff engine walks two directory trees in parallel and classifies every
// path as identical, unique to one side, or conflicting. The apply module
// then produces a merged directory tree based on the user's resolution choices.
package merge

import "fmt"

// ConflictKind describes the relationship between a path in image A and image B.
type ConflictKind int

const (
	// OnlyA indicates the path exists only in image A.
	OnlyA ConflictKind = iota

	// OnlyB indicates the path exists only in image B.
	OnlyB

	// Same indicates the path exists in both images with identical content.
	Same

	// ContentConflict indicates the path exists in both images with different content.
	ContentConflict

	// TypeChange indicates the path is a file in one image and a directory (or
	// other type) in the other.
	TypeChange

	// PermOnly indicates the path has identical content in both images but
	// different permissions, ownership, or other metadata.
	PermOnly

	// BothDeleted indicates the path does not exist in either image (only
	// reachable as a sub-path of a directory that does exist).
	BothDeleted
)

// String returns a human-readable label for the conflict kind.
func (k ConflictKind) String() string {
	switch k {
	case OnlyA:
		return "only-a"
	case OnlyB:
		return "only-b"
	case Same:
		return "same"
	case ContentConflict:
		return "conflict"
	case TypeChange:
		return "type-change"
	case PermOnly:
		return "perm-only"
	case BothDeleted:
		return "both-deleted"
	default:
		return "unknown"
	}
}

// NeedsResolution returns true when the conflict kind requires an explicit
// user or strategy decision to resolve.
func (k ConflictKind) NeedsResolution() bool {
	return k == ContentConflict || k == TypeChange || k == PermOnly
}

// Resolution represents the user's choice for a single conflict.
type Resolution int

const (
	// ResolutionNone means no choice has been made yet.
	ResolutionNone Resolution = iota

	// ResolutionTakeA means the merged image should use image A's version.
	ResolutionTakeA

	// ResolutionTakeB means the merged image should use image B's version.
	ResolutionTakeB

	// ResolutionSkip means the conflict is skipped (defaults to image A).
	ResolutionSkip
)

// String returns a human-readable label for the resolution.
func (r Resolution) String() string {
	switch r {
	case ResolutionNone:
		return "unresolved"
	case ResolutionTakeA:
		return "take-a"
	case ResolutionTakeB:
		return "take-b"
	case ResolutionSkip:
		return "skip"
	default:
		return "unknown"
	}
}

// Conflict represents a single point of disagreement between two images,
// including its path, classification, current resolution, and file metadata.
type Conflict struct {
	// Path is the relative path within the filesystem (e.g. "/etc/nginx/nginx.conf").
	Path string

	// Kind classifies the type of difference at this path.
	Kind ConflictKind

	// Resolution is the user's chosen resolution. It starts as ResolutionNone
	// and is set by the TUI or automatic strategy.
	Resolution Resolution

	// InfoA holds metadata about the file in image A, or nil if absent.
	InfoA *FileInfo

	// InfoB holds metadata about the file in image B, or nil if absent.
	InfoB *FileInfo
}

// FileInfo stores metadata about a single file extracted from one of the images.
type FileInfo struct {
	// RelPath is the path relative to the image root.
	RelPath string

	// AbsPath is the absolute path on disk where the file was extracted.
	AbsPath string

	// IsDir is true if the path refers to a directory.
	IsDir bool

	// IsSymlink is true if the path is a symbolic link.
	IsSymlink bool

	// SymlinkTarget holds the link target when IsSymlink is true.
	SymlinkTarget string

	// Size is the file size in bytes.
	Size int64

	// Mode is the file permission bits (e.g. 0644).
	Mode uint32

	// UID is the numeric owner user ID.
	UID int

	// GID is the numeric owner group ID.
	GID int

	// ContentHash is the hex-encoded xxhash of the file content, computed for
	// files that differ between images. Empty for directories and symlinks.
	ContentHash string
}

// Summary returns a one-line description of the conflict suitable for display.
func (c *Conflict) Summary() string {
	switch c.Kind {
	case OnlyA:
		return fmt.Sprintf("%s (only in A)", c.Path)
	case OnlyB:
		return fmt.Sprintf("%s (only in B)", c.Path)
	case Same:
		return fmt.Sprintf("%s (identical)", c.Path)
	case ContentConflict:
		return fmt.Sprintf("%s (content conflict)", c.Path)
	case TypeChange:
		return fmt.Sprintf("%s (type changed)", c.Path)
	case PermOnly:
		return fmt.Sprintf("%s (permissions differ)", c.Path)
	case BothDeleted:
		return fmt.Sprintf("%s (deleted in both)", c.Path)
	default:
		return c.Path
	}
}

package merge

import "fmt"

type ConflictKind int

const (
	OnlyA ConflictKind = iota
	OnlyB
	Same
	ContentConflict
	TypeChange
	PermOnly
	BothDeleted
)

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

func (k ConflictKind) NeedsResolution() bool {
	return k == ContentConflict || k == TypeChange || k == PermOnly
}

type Resolution int

const (
	ResolutionNone Resolution = iota
	ResolutionTakeA
	ResolutionTakeB
	ResolutionSkip
)

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

type Conflict struct {
	Path      string
	Kind      ConflictKind
	Resolution Resolution
	InfoA     *FileInfo
	InfoB     *FileInfo
}

type FileInfo struct {
	RelPath    string
	AbsPath    string
	IsDir      bool
	IsSymlink  bool
	SymlinkTarget string
	Size       int64
	Mode       uint32
	UID        int
	GID        int
	ContentHash string
}

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

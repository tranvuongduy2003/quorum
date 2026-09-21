package ingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"
)

var ErrIncompatibleArchive = errors.New("checkpoint archive identity does not match current archive")

type ArchiveIdentity string

type ArchiveMember struct {
	Name  string
	Size  uint64
	CRC32 uint32
}

func ComputeArchiveIdentity(members []ArchiveMember) ArchiveIdentity {
	sorted := make([]ArchiveMember, len(members))
	copy(sorted, members)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Name != sorted[j].Name {
			return sorted[i].Name < sorted[j].Name
		}
		if sorted[i].Size != sorted[j].Size {
			return sorted[i].Size < sorted[j].Size
		}
		return sorted[i].CRC32 < sorted[j].CRC32
	})

	hasher := sha256.New()
	for _, member := range sorted {
		fmt.Fprintf(hasher, "%s:%d:%d\n", member.Name, member.Size, member.CRC32)
	}

	return ArchiveIdentity(hex.EncodeToString(hasher.Sum(nil)))
}

type Checkpoint struct {
	Site           Site
	Table          Table
	ArchiveID      ArchiveIdentity
	SourceOffset   int64
	ConfirmedCount int64
	UpdatedAt      time.Time
}

func NewCheckpoint(
	site Site,
	table Table,
	archiveID ArchiveIdentity,
	sourceOffset int64,
	confirmedCount int64,
	updatedAt time.Time,
) Checkpoint {
	return Checkpoint{
		Site:           site,
		Table:          table,
		ArchiveID:      archiveID,
		SourceOffset:   sourceOffset,
		ConfirmedCount: confirmedCount,
		UpdatedAt:      updatedAt.UTC(),
	}
}

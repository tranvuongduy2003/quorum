package ingestion

import (
	"errors"
	"testing"
	"time"
)

func TestComputeArchiveIdentityIsDeterministicWithoutMutatingInput(t *testing.T) {
	members := []ArchiveMember{
		{Name: "Posts.xml", Size: 20, CRC32: 2},
		{Name: "Users.xml", Size: 10, CRC32: 1},
		{Name: "Posts.xml", Size: 19, CRC32: 3},
	}
	reversed := []ArchiveMember{members[2], members[1], members[0]}

	first := ComputeArchiveIdentity(members)
	second := ComputeArchiveIdentity(reversed)

	if first == "" || len(first) != 64 {
		t.Fatalf("identity = %q", first)
	}
	if first != second {
		t.Fatalf("identities differ: %q != %q", first, second)
	}
	if members[0].Name != "Posts.xml" || members[0].Size != 20 || members[2].Size != 19 {
		t.Fatalf("members mutated: %#v", members)
	}
}

func TestComputeArchiveIdentityChangesWithMemberMetadata(t *testing.T) {
	base := []ArchiveMember{{Name: "Posts.xml", Size: 20, CRC32: 2}}
	tests := []ArchiveMember{
		{Name: "Votes.xml", Size: 20, CRC32: 2},
		{Name: "Posts.xml", Size: 21, CRC32: 2},
		{Name: "Posts.xml", Size: 20, CRC32: 3},
	}

	for _, changed := range tests {
		if ComputeArchiveIdentity(base) == ComputeArchiveIdentity([]ArchiveMember{changed}) {
			t.Fatalf("identity unchanged for %#v", changed)
		}
	}
}

func TestComputeArchiveIdentitySupportsEmptyArchive(t *testing.T) {
	if got := ComputeArchiveIdentity(nil); len(got) != 64 {
		t.Fatalf("identity length = %d, want 64", len(got))
	}
}

func TestNewCheckpointAssignsFieldsAndNormalizesTime(t *testing.T) {
	site := Site("academia.stackexchange.com")
	updatedAt := time.Date(2026, time.September, 21, 10, 30, 0, 0, time.FixedZone("ICT", 7*60*60))

	checkpoint := NewCheckpoint(site, TablePosts, ArchiveIdentity("archive"), 41, 17, updatedAt)

	if checkpoint.Site != site || checkpoint.Table != TablePosts || checkpoint.ArchiveID != "archive" || checkpoint.SourceOffset != 41 || checkpoint.ConfirmedCount != 17 {
		t.Fatalf("checkpoint = %#v", checkpoint)
	}
	if !checkpoint.UpdatedAt.Equal(updatedAt) || checkpoint.UpdatedAt.Location() != time.UTC {
		t.Fatalf("UpdatedAt = %v", checkpoint.UpdatedAt)
	}
}

func TestIncompatibleArchiveSentinelHasStableIdentity(t *testing.T) {
	if !errors.Is(ErrIncompatibleArchive, ErrIncompatibleArchive) {
		t.Fatal("errors.Is() = false")
	}
}

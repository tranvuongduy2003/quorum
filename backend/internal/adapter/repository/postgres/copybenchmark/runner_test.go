package copybenchmark

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestVoteAtDatasetBoundaries(t *testing.T) {
	first := voteAt(0)
	if first.Site != "benchmark.example" || first.ID != 1 || first.PostID != 1 || first.VoteTypeID != 2 || first.UserID != nil || first.BountyAmount == nil || *first.BountyAmount != 50 || first.SourceOffset != 0 {
		t.Fatalf("first row = %+v", first)
	}
	if !first.CreatedAt.Equal(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("first timestamp = %s", first.CreatedAt)
	}
	last := voteAt(999_999)
	if last.ID != 1_000_000 || last.PostID != 100_000 || last.VoteTypeID != 3 || last.UserID == nil || *last.UserID != 500_000 || last.BountyAmount != nil || last.SourceOffset != 63_999_936 {
		t.Fatalf("last row = %+v", last)
	}
	wantTime := time.Date(2026, time.January, 1, 0, 16, 39, 999_000_000, time.UTC)
	if !last.CreatedAt.Equal(wantTime) {
		t.Fatalf("last timestamp = %s, want %s", last.CreatedAt, wantTime)
	}
}

func TestVoteValuesPreserveTypedNullables(t *testing.T) {
	without := voteAt(10)
	values := without.values()
	if len(values) != 8 || values[4] != nil || values[6] != nil {
		t.Fatalf("nullable values = %#v", values)
	}
	with := voteAt(1)
	values = with.values()
	if value, ok := values[4].(int64); !ok || value != 2 {
		t.Fatalf("user value = %#v", values[4])
	}
}

func TestCanonicalChecksumDistinguishesNullAndZero(t *testing.T) {
	nullRow := voteAt(10)
	zeroRow := nullRow
	zero := int64(0)
	zeroRow.UserID = &zero
	nullChecksum := canonicalChecksum(t, nullRow)
	zeroChecksum := canonicalChecksum(t, zeroRow)
	if nullChecksum == zeroChecksum {
		t.Fatalf("null and zero checksum = %s", nullChecksum)
	}
	first, err := sourceChecksum(32)
	if err != nil {
		t.Fatalf("sourceChecksum() error = %v", err)
	}
	second, err := sourceChecksum(32)
	if err != nil {
		t.Fatalf("sourceChecksum() second error = %v", err)
	}
	if first != second || len(first) != 64 || first != strings.ToLower(first) {
		t.Fatalf("source checksums = %q/%q", first, second)
	}
}

func TestBatchedInsertSQLUsesExactPlaceholderShape(t *testing.T) {
	full := batchedInsertSQL(1000)
	if strings.Count(full, "$") != 8000 || !strings.Contains(full, "($7993,$7994,$7995,$7996,$7997,$7998,$7999,$8000)") {
		t.Fatalf("full batch placeholder shape is invalid")
	}
	tail := batchedInsertSQL(2)
	if strings.Count(tail, "$") != 16 || !strings.HasSuffix(tail, "($9,$10,$11,$12,$13,$14,$15,$16)") {
		t.Fatalf("tail query = %s", tail)
	}
}

func TestTextRowEscapesCopySyntaxAndNulls(t *testing.T) {
	row := voteAt(10)
	row.Site = "a\\b\tc\rd\ne"
	got := textRow(row)
	wantPrefix := "a\\\\b\\tc\\rd\\ne\t11\t11\t2\t\\N\t"
	if !strings.HasPrefix(got, wantPrefix) || !strings.Contains(got, "\t\\N\t640\n") {
		t.Fatalf("text row = %q", got)
	}
}

func canonicalChecksum(t *testing.T, row voteRow) string {
	t.Helper()
	h := sha256.New()
	if err := writeCanonical(h, row); err != nil {
		t.Fatalf("writeCanonical() error = %v", err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

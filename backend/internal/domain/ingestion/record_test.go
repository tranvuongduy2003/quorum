package ingestion

import (
	"testing"
	"time"
)

func TestNewQuarantineRecordCopiesSourceAndNormalizesTime(t *testing.T) {
	attributes := map[string]string{"Id": "1"}
	source := NewSourceRecord(TablePosts, 17, `<row Id="1" />`, attributes)
	foundAt := time.Date(2026, time.September, 6, 10, 0, 0, 0, time.FixedZone("ICT", 7*60*60))

	record := NewQuarantineRecord(Site("stackoverflow.com"), source, ReasonWatermarkPattern, foundAt)
	source.Attributes["Id"] = "2"

	if got, _ := record.Source.Attribute("Id"); got != "1" {
		t.Fatalf("source attribute = %q, want %q", got, "1")
	}
	if record.Source.Table != TablePosts || record.Source.Offset != 17 || record.Source.Raw != `<row Id="1" />` {
		t.Fatalf("source = %#v", record.Source)
	}
	if got, want := record.FoundAt, foundAt.UTC(); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("found at = %v, want %v in UTC", got, want)
	}
}

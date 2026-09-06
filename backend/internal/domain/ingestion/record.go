package ingestion

import (
	"maps"
	"time"
)

type SourceRecord struct {
	Table      Table
	Offset     int64
	Raw        string
	Attributes map[string]string
}

func NewSourceRecord(table Table, offset int64, raw string, attributes map[string]string) SourceRecord {
	return SourceRecord{
		Table:      table,
		Offset:     offset,
		Raw:        raw,
		Attributes: maps.Clone(attributes),
	}
}

func (r SourceRecord) Attribute(name string) (string, bool) {
	attribute, ok := r.Attributes[name]
	return attribute, ok
}

type QuarantineRecord struct {
	Site    Site
	Source  SourceRecord
	Reason  ReasonCode
	FoundAt time.Time
}

func NewQuarantineRecord(
	site Site,
	source SourceRecord,
	reason ReasonCode,
	foundAt time.Time,
) QuarantineRecord {
	return QuarantineRecord{
		Site:    site,
		Source:  NewSourceRecord(source.Table, source.Offset, source.Raw, source.Attributes),
		Reason:  reason,
		FoundAt: foundAt.UTC(),
	}
}

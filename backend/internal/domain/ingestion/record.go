package ingestion

import (
	"maps"
)

type ReasonCode string

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

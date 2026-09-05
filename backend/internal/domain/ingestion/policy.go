package ingestion

import (
	"strings"
	"time"
)

type ReasonCode string

const ReasonInvalidTimestamp ReasonCode = "invalid_timestamp"
const ReasonWatermarkPattern ReasonCode = "watermark_pattern"

type Finding struct {
	Reason ReasonCode
}

type Policy interface {
	Reason() ReasonCode
	Reject(record SourceRecord) bool
}

func FirstFinding(record SourceRecord, policies []Policy) (Finding, bool) {
	for _, policy := range policies {
		if policy.Reject(record) {
			return Finding{Reason: policy.Reason()}, true
		}
	}

	return Finding{}, false
}

type TimestampPolicy struct {
	attributes []string
}

func NewTimestampPolicy() TimestampPolicy {
	return TimestampPolicy{
		attributes: []string{
			"CreationDate",
			"LastEditDate",
			"LastActivityDate",
			"ClosedDate",
			"CommunityOwnedDate",
			"LastAccessDate",
			"Date",
		},
	}
}

func (TimestampPolicy) Reason() ReasonCode {
	return ReasonInvalidTimestamp
}

func (p TimestampPolicy) Reject(record SourceRecord) bool {
	for _, attribute := range p.attributes {
		value, ok := record.Attribute(attribute)
		if !ok || value == "" {
			continue
		}

		if _, err := time.Parse("2006-01-02T15:04:05.000", value); err != nil {
			return true
		}
	}

	return false
}

type WatermarkPolicy struct {
	patterns []string
}

func NewWatermarkPolicy(patterns []string) (WatermarkPolicy, error) {
	seen := make(map[string]struct{}, len(patterns))
	normalized := make([]string, 0, len(patterns))

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if _, ok := seen[pattern]; ok {
			continue
		}

		seen[pattern] = struct{}{}
		normalized = append(normalized, pattern)
	}

	if len(normalized) == 0 {
		return WatermarkPolicy{}, ErrEmptyWatermarkPatterns
	}

	return WatermarkPolicy{patterns: normalized}, nil
}

func (WatermarkPolicy) Reason() ReasonCode {
	return ReasonWatermarkPattern
}

func (p WatermarkPolicy) Reject(record SourceRecord) bool {
	for _, pattern := range p.patterns {
		if strings.Contains(record.Raw, pattern) {
			return true
		}
	}

	return false
}

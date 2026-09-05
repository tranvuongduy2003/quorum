package ingestion

import (
	"errors"
	"testing"
)

func TestTimestampPolicyRejectsEveryKnownInvalidTimestamp(t *testing.T) {
	attributes := []string{
		"CreationDate",
		"LastEditDate",
		"LastActivityDate",
		"ClosedDate",
		"CommunityOwnedDate",
		"LastAccessDate",
		"Date",
	}
	policy := NewTimestampPolicy()

	for _, attribute := range attributes {
		t.Run(attribute, func(t *testing.T) {
			record := NewSourceRecord(TablePosts, 1, "<row />", map[string]string{attribute: "not-a-date"})
			if !policy.Reject(record) {
				t.Fatalf("Reject() = false for invalid %s", attribute)
			}
		})
	}
}

func TestTimestampPolicyAcceptsMissingEmptyAndExactTimestamps(t *testing.T) {
	policy := NewTimestampPolicy()
	record := NewSourceRecord(TablePosts, 1, "<row />", map[string]string{
		"CreationDate": "2026-09-06T14:25:31.042",
		"LastEditDate": "",
	})

	if policy.Reject(record) {
		t.Fatal("Reject() = true, want false")
	}
}

func TestNewWatermarkPolicyNormalizesCopiesAndMatchesLiterally(t *testing.T) {
	patterns := []string{" synthetic-marker ", "", "synthetic-marker", "CaseSensitive"}
	policy, err := NewWatermarkPolicy(patterns)
	if err != nil {
		t.Fatalf("NewWatermarkPolicy() error = %v", err)
	}
	patterns[0] = "changed"

	if got, want := len(policy.patterns), 2; got != want {
		t.Fatalf("patterns length = %d, want %d", got, want)
	}
	if !policy.Reject(NewSourceRecord(TablePosts, 1, "prefix synthetic-marker suffix", nil)) {
		t.Fatal("Reject() = false for literal match")
	}
	if policy.Reject(NewSourceRecord(TablePosts, 1, "prefix casesensitive suffix", nil)) {
		t.Fatal("Reject() = true for case-mismatched text")
	}
}

func TestNewWatermarkPolicyRejectsEmptyPatterns(t *testing.T) {
	_, err := NewWatermarkPolicy([]string{" ", "\t"})
	if !errors.Is(err, ErrEmptyWatermarkPatterns) {
		t.Fatalf("NewWatermarkPolicy() error = %v", err)
	}
}

func TestFirstFindingUsesPolicyOrder(t *testing.T) {
	timestamp := NewTimestampPolicy()
	watermark, err := NewWatermarkPolicy([]string{"synthetic-marker"})
	if err != nil {
		t.Fatalf("NewWatermarkPolicy() error = %v", err)
	}
	record := NewSourceRecord(TablePosts, 1, "<row Body=\"synthetic-marker\" />", map[string]string{
		"CreationDate": "not-a-date",
	})

	finding, ok := FirstFinding(record, []Policy{timestamp, watermark})
	if !ok || finding.Reason != ReasonInvalidTimestamp {
		t.Fatalf("FirstFinding() = %#v, %t", finding, ok)
	}
}

func TestFirstFindingReturnsZeroFindingWhenAccepted(t *testing.T) {
	finding, ok := FirstFinding(NewSourceRecord(TablePosts, 1, "<row />", nil), []Policy{NewTimestampPolicy()})
	if ok || finding != (Finding{}) {
		t.Fatalf("FirstFinding() = %#v, %t", finding, ok)
	}
}

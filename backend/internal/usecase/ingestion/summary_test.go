package ingestion

import (
	"bytes"
	"errors"
	"testing"

	domainingestion "quorum/internal/domain/ingestion"
)

func TestPrintSummarySortsRejectionReasons(t *testing.T) {
	summary := RunSummary{
		Tables: []TableSummary{
			{
				Table:     domainingestion.TablePosts,
				Processed: 3,
				Valid:     1,
				Rejected:  2,
				Rejections: map[domainingestion.ReasonCode]int64{
					domainingestion.ReasonWatermarkPattern: 1,
					domainingestion.ReasonInvalidTimestamp: 1,
				},
			},
		},
		DryRun:           true,
		ThresholdPercent: 100,
		Status:           RunStatusOK,
	}
	var output bytes.Buffer

	PrintSummary(&output, summary)

	want := "table=posts processed=3 valid=1 rejected=2 malformed=0\n" +
		"rejection table=posts reason=invalid_timestamp count=1\n" +
		"rejection table=posts reason=watermark_pattern count=1\n" +
		"status=ok dry_run=true threshold_percent=100.0000 observed_percent=0.0000\n"
	if got := output.String(); got != want {
		t.Fatalf("PrintSummary() = %q, want %q", got, want)
	}
}

func TestTableSummaryRejectedPercent(t *testing.T) {
	tests := []struct {
		name    string
		summary TableSummary
		want    float64
	}{
		{name: "empty", summary: TableSummary{}, want: 0},
		{name: "fractional", summary: TableSummary{Processed: 1000, Rejected: 1}, want: 0.1},
		{name: "half percent", summary: TableSummary{Processed: 200, Rejected: 1}, want: 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.summary.RejectedPercent(); got != tt.want {
				t.Fatalf("RejectedPercent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunSummaryPercentageMethodsUsePerTableRatesAndSelectionOrder(t *testing.T) {
	summary := RunSummary{Tables: []TableSummary{
		{Table: domainingestion.TablePosts, Processed: 1000, Rejected: 1},
		{Table: domainingestion.TableTags, Processed: 200, Rejected: 1},
		{Table: domainingestion.TableUsers, Processed: 10, Rejected: 1},
	}}

	if got, want := summary.MaxRejectedPercent(), 10.0; got != want {
		t.Fatalf("MaxRejectedPercent() = %v, want %v", got, want)
	}
	first, ok := summary.FirstTableAbove(0.4)
	if !ok || first.Table != domainingestion.TableTags {
		t.Fatalf("FirstTableAbove() = %#v, %t", first, ok)
	}
}

func TestRunSummaryFirstTableAboveUsesStrictComparison(t *testing.T) {
	summary := RunSummary{Tables: []TableSummary{
		{Table: domainingestion.TablePosts, Processed: 200, Rejected: 1},
	}}

	if table, ok := summary.FirstTableAbove(0.5); ok || table.Table != "" || table.Processed != 0 {
		t.Fatalf("FirstTableAbove(0.5) = %#v, %t", table, ok)
	}
	if table, ok := summary.FirstTableAbove(0.4999); !ok || table.Table != domainingestion.TablePosts {
		t.Fatalf("FirstTableAbove(0.4999) = %#v, %t", table, ok)
	}
}

func TestRejectThresholdErrorFormatsAndClassifies(t *testing.T) {
	err := RejectThresholdError{ObservedPercent: 0.5001, ThresholdPercent: 0.5}

	if got, want := err.Error(), "rejected record percentage 0.5001 exceeds configured threshold 0.5000"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, ErrRejectThresholdExceeded) {
		t.Fatal("errors.Is(err, ErrRejectThresholdExceeded) = false")
	}
}

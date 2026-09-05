package ingestion

import (
	"fmt"
	"io"
	domainingestion "quorum/internal/domain/ingestion"
	"sort"
)

type RunStatus string

const RunStatusOK RunStatus = "ok"
const RunStatusFailed RunStatus = "failed"

type TableSummary struct {
	Table      domainingestion.Table
	Processed  int64
	Valid      int64
	Rejected   int64
	Malformed  int64
	Rejections map[domainingestion.ReasonCode]int64
	LastOffset int64
}

type RunSummary struct {
	Tables           []TableSummary
	DryRun           bool
	ThresholdPercent float64
	ObservedPercent  float64
	Status           RunStatus
}

func (s TableSummary) RejectedPercent() float64 {
	if s.Processed == 0 {
		return 0
	}

	return float64(s.Rejected) / float64(s.Processed) * 100
}

func (s RunSummary) MaxRejectedPercent() float64 {
	var maximum float64
	for _, table := range s.Tables {
		if observed := table.RejectedPercent(); observed > maximum {
			maximum = observed
		}
	}

	return maximum
}

func (s RunSummary) FirstTableAbove(threshold float64) (TableSummary, bool) {
	for _, table := range s.Tables {
		if table.RejectedPercent() > threshold {
			return table, true
		}
	}

	return TableSummary{}, false
}

func (s RunSummary) WithStatus(status RunStatus) RunSummary {
	s.Status = status
	return s
}
func PrintSummary(w io.Writer, summary RunSummary) {
	for _, t := range summary.Tables {
		fmt.Fprintf(w, "table=%v processed=%d valid=%d rejected=%d malformed=%d\n",
			t.Table, t.Processed, t.Valid, t.Rejected, t.Malformed)

		reasons := make([]string, 0, len(t.Rejections))
		for reason := range t.Rejections {
			reasons = append(reasons, string(reason))
		}
		sort.Strings(reasons)

		for _, reason := range reasons {
			fmt.Fprintf(w, "rejection table=%s reason=%s count=%d\n",
				t.Table, reason, t.Rejections[domainingestion.ReasonCode(reason)])
		}
	}

	fmt.Fprintf(w, "status=%s dry_run=%t threshold_percent=%.4f observed_percent=%.4f\n",
		summary.Status, summary.DryRun, summary.ThresholdPercent, summary.ObservedPercent)
}

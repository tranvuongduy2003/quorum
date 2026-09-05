package ingestion

import (
	"fmt"
	"io"
	domainingestion "quorum/internal/domain/ingestion"
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

func (s RunSummary) WithStatus(status RunStatus) RunSummary {
	s.Status = status
	return s
}
func PrintSummary(w io.Writer, summary RunSummary) {
	for _, t := range summary.Tables {
		fmt.Fprintf(w, "table=%v processed=%d valid=%d rejected=%d malformed=%d\n",
			t.Table, t.Processed, t.Valid, t.Rejected, t.Malformed)
	}

	fmt.Fprintf(w, "status=%s dry_run=%t threshold_percent=%.4f observed_percent=%.4f\n",
		summary.Status, summary.DryRun, summary.ThresholdPercent, summary.ObservedPercent)
}

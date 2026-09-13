package ingestion

import (
	"context"
	"errors"
	"io"
	"time"

	domainingestion "quorum/internal/domain/ingestion"
)

const copyBatchRows = 4096
const copyBatchRawBytes int64 = 32 * 1024 * 1024

type pendingWrites struct {
	accepted   []domainingestion.SourceRecord
	quarantine []domainingestion.QuarantineRecord
	rawBytes   int64
}

type Service struct {
	archives ArchiveFactory
	writer   Writer
	now      func() time.Time
	policies []domainingestion.Policy
}

func NewService(
	archives ArchiveFactory,
	writer Writer,
	now func() time.Time,
	policies ...domainingestion.Policy,
) Service {
	if now == nil {
		now = time.Now
	}

	return Service{
		archives: archives,
		writer:   writer,
		now:      now,
		policies: append([]domainingestion.Policy(nil), policies...),
	}
}
func (s Service) Run(ctx context.Context, command Command) (summary RunSummary, runErr error) {
	if !command.DryRun && s.writer == nil {
		summary = summary.WithStatus(RunStatusFailed)
		return summary, RunError{Table: "request", Offset: 0, Err: ErrWriterRequired}
	}

	archive, err := s.openValidated(ctx, command)
	if err != nil {
		return RunSummary{}, err
	}
	defer func() {
		if closeErr := archive.Close(); closeErr != nil && runErr == nil {
			summary = summary.WithStatus(RunStatusFailed)
			runErr = RunError{Table: command.Tables[0].String(), Offset: 0, Err: closeErr}
		}
	}()

	summary = RunSummary{
		DryRun:           command.DryRun,
		ThresholdPercent: command.RejectThresholdPercent,
		Status:           RunStatusOK,
		Tables:           make([]TableSummary, 0, len(command.Tables)),
	}

	for _, table := range command.Tables {
		tSum, tableErr := s.processTable(ctx, archive, table, command)

		summary.Tables = append(summary.Tables, tSum)
		summary.ObservedPercent = summary.MaxRejectedPercent()

		if tableErr != nil {
			return summary.WithStatus(RunStatusFailed), tableErr
		}
	}

	failingTable, exceeded := summary.FirstTableAbove(command.RejectThresholdPercent)
	if exceeded {
		summary.Status = RunStatusFailed
		return summary, RunError{
			Table:  failingTable.Table.String(),
			Offset: failingTable.LastOffset,
			Err: RejectThresholdError{
				ObservedPercent:  failingTable.RejectedPercent(),
				ThresholdPercent: command.RejectThresholdPercent,
			},
		}
	}

	return summary, nil
}

func (s Service) openValidated(ctx context.Context, command Command) (Archive, error) {
	if err := ctx.Err(); err != nil {
		return nil, RunError{Table: command.Tables[0].String(), Offset: 0, Err: err}
	}

	archive, err := s.archives.Open(command.ArchivePath)
	if err != nil {
		return nil, RunError{Table: command.Tables[0].String(), Offset: 0, Err: err}
	}

	if err := archive.ValidateTables(command.Tables); err != nil {
		_ = archive.Close()
		if sourceErr, ok := sourceError(err); ok {
			return nil, RunError{Table: sourceErr.Table.String(), Offset: sourceErr.Offset, Err: sourceErr.Err}
		}
		return nil, RunError{Table: command.Tables[0].String(), Offset: 0, Err: err}
	}

	return archive, nil
}

func (s Service) processTable(ctx context.Context, archive Archive, table domainingestion.Table, command Command) (summary TableSummary, runErr error) {
	pending := pendingWrites{
		accepted:   make([]domainingestion.SourceRecord, 0, copyBatchRows),
		quarantine: make([]domainingestion.QuarantineRecord, 0, copyBatchRows),
		rawBytes:   0,
	}

	summary = TableSummary{
		Table:      table,
		Rejections: make(map[domainingestion.ReasonCode]int64),
	}

	member, err := archive.OpenTable(ctx, table, command.MaxRecordBytes)
	if err != nil {
		var sourceErr SourceError
		if errors.As(err, &sourceErr) {
			return summary, RunError{
				Table:  sourceErr.Table.String(),
				Offset: sourceErr.Offset,
				Err:    sourceErr.Err,
			}
		}
		return summary, RunError{
			Table:  table.String(),
			Offset: 0,
			Err:    err,
		}
	}
	defer func() {
		if closeErr := member.Close(); closeErr != nil && runErr == nil {
			runErr = RunError{Table: table.String(), Offset: summary.LastOffset, Err: closeErr}
		}
	}()

	for {
		record, err := member.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = s.flushPending(ctx, command.Site, table, &pending, &summary)
				if err != nil {
					return summary, err
				}
				break
			}

			var sourceErr SourceError
			if errors.As(err, &sourceErr) {
				summary.Processed++
				summary.Malformed++
				summary.LastOffset = sourceErr.Offset
				return summary, RunError{
					Table:  sourceErr.Table.String(),
					Offset: sourceErr.Offset,
					Err:    sourceErr.Err,
				}
			}

			return summary, RunError{
				Table:  table.String(),
				Offset: summary.LastOffset,
				Err:    err,
			}
		}

		summary.Processed++
		summary.LastOffset = record.Offset

		finding, rejected := domainingestion.FirstFinding(record, s.policies)
		if rejected {
			summary.Rejected++
			summary.Rejections[finding.Reason]++
		} else {
			summary.Valid++
		}

		if command.DryRun {
			continue
		}

		if pending.shouldFlush(record) {
			err = s.flushPending(ctx, command.Site, table, &pending, &summary)
			if err != nil {
				return summary, err
			}
		}

		if !rejected {
			pending.accepted = append(pending.accepted, record)
		} else {
			quarantineRecord := domainingestion.NewQuarantineRecord(command.Site, record, finding.Reason, s.now())
			pending.quarantine = append(pending.quarantine, quarantineRecord)
		}

		pending.rawBytes += int64(len(record.Raw))

		if pending.count() >= copyBatchRows || pending.rawBytes >= copyBatchRawBytes {
			err = s.flushPending(ctx, command.Site, table, &pending, &summary)
			if err != nil {
				return summary, err
			}
		}
	}

	return summary, nil
}

func (s Service) flushPending(
	ctx context.Context,
	site domainingestion.Site,
	table domainingestion.Table,
	pending *pendingWrites,
	summary *TableSummary,
) error {
	if len(pending.accepted) > 0 {
		offset := pending.accepted[len(pending.accepted)-1].Offset
		count, err := s.writer.WriteAccepted(ctx, site, table, pending.accepted)

		if err != nil {
			return RunError{
				Table:  table.String(),
				Offset: offset,
				Err:    err,
			}
		}

		if count != int64(len(pending.accepted)) {
			return RunError{
				Table:  table.String(),
				Offset: offset,
				Err:    ErrWriteCountMismatch,
			}
		}

		summary.Confirmed += count
	}

	if len(pending.quarantine) > 0 {
		offset := pending.quarantine[len(pending.quarantine)-1].Source.Offset
		count, err := s.writer.WriteQuarantine(ctx, pending.quarantine)

		if err != nil {
			return RunError{
				Table:  table.String(),
				Offset: offset,
				Err:    err,
			}
		}

		if count != int64(len(pending.quarantine)) {
			return RunError{
				Table:  table.String(),
				Offset: offset,
				Err:    ErrWriteCountMismatch,
			}
		}

		summary.Confirmed += count
	}

	pending.reset()

	return nil
}

func sourceError(err error) (SourceError, bool) {
	var sourceErr SourceError
	return sourceErr, errors.As(err, &sourceErr)
}

func (p pendingWrites) count() int {
	return len(p.accepted) + len(p.quarantine)
}

func (p pendingWrites) shouldFlush(next domainingestion.SourceRecord) bool {
	currentCount := p.count()
	if currentCount == 0 {
		return false
	}

	if currentCount+1 > copyBatchRows {
		return true
	}

	if p.rawBytes+int64(len(next.Raw)) > copyBatchRawBytes {
		return true
	}

	return false
}

func (p *pendingWrites) reset() {
	p.accepted = p.accepted[:0]
	p.quarantine = p.quarantine[:0]
	p.rawBytes = 0
}
